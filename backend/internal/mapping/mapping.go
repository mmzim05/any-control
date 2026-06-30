package mapping

import (
	"math"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/simlink/internal/input"
)

const MaxChannels = 32

type Transform struct {
	Scale    float64 `json:"scale"`    // 1.0 default
	Offset   float64 `json:"offset"`   // 0.0 default
	Deadzone float64 `json:"deadzone"` // 0.0–1.0
	Expo     float64 `json:"expo"`     // 0.0–1.0
	Reverse  bool    `json:"reverse"`
}

type Rule struct {
	DeviceID  string          `json:"device_id"`
	Code      uint16          `json:"code"`
	EventType input.EventType `json:"event_type"`
	Channel   int             `json:"channel"` // 0-indexed, 0–31
	Transform Transform       `json:"transform"`
	Failsafe  float64         `json:"failsafe"` // sent on link loss, normalized -1.0..1.0
}

type channelState struct {
	values [MaxChannels]float64
}

type Engine struct {
	mu      sync.RWMutex
	rules   []Rule
	axisMin map[string]int32 // cached per device+code
	axisMax map[string]int32
	state   unsafe.Pointer // *channelState, atomic
}

func NewEngine() *Engine {
	s := &channelState{}
	e := &Engine{
		axisMin: make(map[string]int32),
		axisMax: make(map[string]int32),
	}
	atomic.StorePointer(&e.state, unsafe.Pointer(s))
	return e
}

func (e *Engine) SetRules(rules []Rule) {
	e.mu.Lock()
	e.rules = make([]Rule, len(rules))
	copy(e.rules, rules)
	e.mu.Unlock()
}

func (e *Engine) Rules() []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Rule, len(e.rules))
	copy(out, e.rules)
	return out
}

// SetAxisRange informs the engine of a device axis's min/max for normalization.
func (e *Engine) SetAxisRange(deviceID string, code uint16, min, max int32) {
	key := axisKey(deviceID, code)
	e.mu.Lock()
	e.axisMin[key] = min
	e.axisMax[key] = max
	e.mu.Unlock()
}

// Process applies an input event to the channel state.
func (e *Engine) Process(ev input.Event) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for i := range e.rules {
		r := &e.rules[i]
		if r.DeviceID != ev.DeviceID || r.Code != ev.Code || r.EventType != ev.Type {
			continue
		}
		if r.Channel < 0 || r.Channel >= MaxChannels {
			continue
		}

		var normalized float64
		switch ev.Type {
		case input.EventAxis:
			normalized = e.normalizeAxis(ev.DeviceID, ev.Code, ev.Value)
		case input.EventButton:
			if ev.Value > 0 {
				normalized = 1.0
			} else {
				normalized = -1.0
			}
		}

		normalized = applyTransform(normalized, r.Transform)

		// Atomic swap of the state snapshot
		for {
			old := (*channelState)(atomic.LoadPointer(&e.state))
			next := *old
			next.values[r.Channel] = normalized
			if atomic.CompareAndSwapPointer(&e.state, unsafe.Pointer(old), unsafe.Pointer(&next)) {
				break
			}
		}
	}
}

// Channels returns a snapshot of all channel values.
func (e *Engine) Channels() [MaxChannels]float64 {
	s := (*channelState)(atomic.LoadPointer(&e.state))
	return s.values
}

// Failsafe builds a failsafe array from current rules. Channels with no rule are 0.
func (e *Engine) Failsafe() [MaxChannels]float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var fs [MaxChannels]float64
	for _, r := range e.rules {
		if r.Channel >= 0 && r.Channel < MaxChannels {
			fs[r.Channel] = r.Failsafe
		}
	}
	return fs
}

func (e *Engine) normalizeAxis(deviceID string, code uint16, value int32) float64 {
	key := axisKey(deviceID, code)
	min := e.axisMin[key]
	max := e.axisMax[key]
	if max == min {
		return 0
	}
	// Map [min, max] → [-1.0, 1.0]
	return 2.0*float64(value-min)/float64(max-min) - 1.0
}

func applyTransform(v float64, t Transform) float64 {
	if t.Reverse {
		v = -v
	}
	// Deadzone
	if t.Deadzone > 0 {
		if math.Abs(v) < t.Deadzone {
			v = 0
		} else {
			sign := math.Copysign(1, v)
			v = sign * (math.Abs(v) - t.Deadzone) / (1.0 - t.Deadzone)
		}
	}
	// Expo: v' = v * (expo * v^2 + (1 - expo))
	if t.Expo > 0 {
		v = v * (t.Expo*v*v + (1.0 - t.Expo))
	}
	v = v*t.Scale + t.Offset
	if v > 1.0 {
		v = 1.0
	}
	if v < -1.0 {
		v = -1.0
	}
	return v
}

func axisKey(deviceID string, code uint16) string {
	// Pre-allocate friendly key without fmt.Sprintf
	b := make([]byte, 0, len(deviceID)+6)
	b = append(b, deviceID...)
	b = append(b, ':')
	b = appendUint16(b, code)
	return string(b)
}

func appendUint16(b []byte, v uint16) []byte {
	return append(b, byte(v>>8), byte(v))
}
