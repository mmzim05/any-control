package input

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	evdev "github.com/holoplot/go-evdev"
)

type EventType int

const (
	EventAxis   EventType = 0
	EventButton EventType = 1
)

type Event struct {
	DeviceID string
	Code     uint16
	Type     EventType
	Value    int32
}

type AxisInfo struct {
	Code int    `json:"code"`
	Name string `json:"name"`
	Min  int32  `json:"min"`
	Max  int32  `json:"max"`
}

type ButtonInfo struct {
	Code int    `json:"code"`
	Name string `json:"name"`
}

type Device struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Vendor  uint16       `json:"vendor"`
	Product uint16       `json:"product"`
	Axes    []AxisInfo   `json:"axes"`
	Buttons []ButtonInfo `json:"buttons"`
}

type deviceWorker struct {
	dev    *evdev.InputDevice
	cancel context.CancelFunc
	info   Device
}

type Manager struct {
	mu      sync.RWMutex
	workers map[string]*deviceWorker
	events  chan Event
}

func NewManager() *Manager {
	return &Manager{
		workers: make(map[string]*deviceWorker),
		events:  make(chan Event, 1024),
	}
}

func (m *Manager) Events() <-chan Event {
	return m.events
}

func (m *Manager) Devices() []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Device, 0, len(m.workers))
	for _, w := range m.workers {
		out = append(out, w.info)
	}
	return out
}

// Run scans for devices every 2 seconds for hotplug support. Blocks until ctx cancelled.
func (m *Manager) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	m.scan(ctx)
	for {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			for _, w := range m.workers {
				w.cancel()
				w.dev.Close()
			}
			m.mu.Unlock()
			return
		case <-ticker.C:
			m.scan(ctx)
		}
	}
}

func (m *Manager) scan(ctx context.Context) {
	paths, _ := filepath.Glob("/dev/input/event*")
	found := make(map[string]bool, len(paths))
	for _, p := range paths {
		found[p] = true
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for path, w := range m.workers {
		if !found[path] {
			log.Printf("input: removed %s (%s)", path, w.info.Name)
			w.cancel()
			w.dev.Close()
			delete(m.workers, path)
		}
	}
	for _, path := range paths {
		if _, exists := m.workers[path]; !exists {
			m.tryAdd(ctx, path)
		}
	}
}

// tryAdd must be called with m.mu held.
func (m *Manager) tryAdd(ctx context.Context, path string) {
	dev, err := evdev.Open(path)
	if err != nil {
		return
	}

	capTypes := dev.CapableTypes()
	hasAbs, hasKey := false, false
	for _, t := range capTypes {
		if t == evdev.EV_ABS {
			hasAbs = true
		}
		if t == evdev.EV_KEY {
			hasKey = true
		}
	}
	if !hasAbs && !hasKey {
		dev.Close()
		return
	}

	// Non-blocking reads so the goroutine can respect context cancellation
	dev.NonBlock()

	info := buildInfo(path, dev)
	wCtx, cancel := context.WithCancel(ctx)
	w := &deviceWorker{dev: dev, cancel: cancel, info: info}
	m.workers[path] = w

	log.Printf("input: added %s (%s) axes=%d buttons=%d", path, info.Name, len(info.Axes), len(info.Buttons))
	go m.readLoop(wCtx, path, w)
}

func (m *Manager) readLoop(ctx context.Context, path string, w *deviceWorker) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ev, err := w.dev.ReadOne()
		if err != nil {
			if errors.Is(err, syscall.EAGAIN) {
				// No event ready; brief sleep to avoid hot spin
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Millisecond):
				}
				continue
			}
			// Real error — device gone
			log.Printf("input: read error %s: %v", path, err)
			m.mu.Lock()
			if w2, ok := m.workers[path]; ok && w2 == w {
				w.cancel()
				w.dev.Close()
				delete(m.workers, path)
			}
			m.mu.Unlock()
			return
		}

		var out Event
		switch ev.Type {
		case evdev.EV_ABS:
			out = Event{DeviceID: path, Code: uint16(ev.Code), Type: EventAxis, Value: ev.Value}
		case evdev.EV_KEY:
			out = Event{DeviceID: path, Code: uint16(ev.Code), Type: EventButton, Value: ev.Value}
		default:
			continue
		}

		select {
		case m.events <- out:
		case <-ctx.Done():
			return
		}
	}
}

func buildInfo(path string, dev *evdev.InputDevice) Device {
	name, _ := dev.Name()
	id, _ := dev.InputID()

	info := Device{
		ID:      path,
		Name:    strings.TrimSpace(name),
		Vendor:  id.Vendor,
		Product: id.Product,
	}

	// Axes
	if absMap, err := dev.AbsInfos(); err == nil {
		for code, abs := range absMap {
			axisName := evdev.ABSToString[code]
			if axisName == "" {
				axisName = "ABS_UNKNOWN"
			}
			info.Axes = append(info.Axes, AxisInfo{
				Code: int(code),
				Name: axisName,
				Min:  abs.Minimum,
				Max:  abs.Maximum,
			})
		}
	}

	// Buttons: enumerate all capable KEY codes in the button range
	for _, code := range dev.CapableEvents(evdev.EV_KEY) {
		// Button codes start at 0x100 (BTN_MISC); skip keyboard keys below that
		if code < 0x100 {
			continue
		}
		btnName := evdev.KEYToString[code]
		if btnName == "" {
			btnName = "BTN_UNKNOWN"
		}
		info.Buttons = append(info.Buttons, ButtonInfo{
			Code: int(code),
			Name: btnName,
		})
	}

	return info
}
