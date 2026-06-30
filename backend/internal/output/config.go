package output

import (
	"context"
	"log"
	"sync"

	"github.com/simlink/internal/mapping"
	"github.com/simlink/internal/output/crsf"
	"github.com/simlink/internal/output/ppm"
	"github.com/simlink/internal/output/sbus"
)

type Protocol string

const (
	ProtocolNone Protocol = ""
	ProtocolCRSF Protocol = "crsf"
	ProtocolSBUS Protocol = "sbus"
	ProtocolPPM  Protocol = "ppm"
)

type Config struct {
	Protocol    Protocol   `json:"protocol"`
	SerialPort  string     `json:"serial_port"`
	AudioDevice string     `json:"audio_device"`
	Failsafe    [32]float64 `json:"failsafe"` // normalized [-1.0, 1.0]
	Enabled     bool       `json:"enabled"`
}

type Manager struct {
	mu      sync.Mutex
	cfg     Config
	cancel  context.CancelFunc
	engine  *mapping.Engine
}

func NewManager(engine *mapping.Engine) *Manager {
	return &Manager{engine: engine}
}

func (m *Manager) GetConfig() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

func (m *Manager) SetConfig(cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop current output
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}

	m.cfg = cfg
	if !cfg.Enabled {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	channels := func() [32]float64 {
		return m.engine.Channels()
	}

	switch cfg.Protocol {
	case ProtocolCRSF:
		var fs [16]uint16
		for i := 0; i < 16; i++ {
			v := cfg.Failsafe[i]
			fs[i] = uint16((v+1.0)/2.0*float64(1811-172) + 172)
		}
		o := crsf.New(cfg.SerialPort, fs)
		go func() {
			o.Run(ctx, channels)
			log.Println("output: crsf stopped")
		}()

	case ProtocolSBUS:
		var fs [16]uint16
		for i := 0; i < 16; i++ {
			v := cfg.Failsafe[i]
			fs[i] = uint16((v+1.0)/2.0*float64(1811-172) + 172)
		}
		o := sbus.New(cfg.SerialPort, fs)
		go func() {
			o.Run(ctx, channels)
			log.Println("output: sbus stopped")
		}()

	case ProtocolPPM:
		o := ppm.New(cfg.AudioDevice)
		go func() {
			o.Run(ctx, channels)
			log.Println("output: ppm stopped")
		}()

	default:
		cancel()
		m.cancel = nil
	}
}

func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}
