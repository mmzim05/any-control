// Package sbus outputs SBUS frames over serial.
// Protocol: 100000 baud 8E2 (inverted). Requires hardware signal inverter.
// 25-byte frame: 0x0F start, 22 payload bytes (16ch × 11-bit packed), flags, 0x00 end.
package sbus

import (
	"context"
	"log"
	"time"

	serial "go.bug.st/serial"
)

const (
	baudRate    = 100000
	startByte   = 0x0F
	endByte     = 0x00
	numChannels = 16
	tickRate    = 50 // Hz
)

type Output struct {
	port     string
	failsafe [numChannels]uint16
}

func New(port string, failsafe [numChannels]uint16) *Output {
	return &Output{port: port, failsafe: failsafe}
}

// Run opens the serial port and sends SBUS frames until ctx is cancelled.
// NOTE: SBUS requires an inverted signal level. Use a hardware inverter between
// the serial TX pin and the receiver. Some USB-UART adapters (e.g. CP2102N with
// firmware configuration) support software inversion.
func (o *Output) Run(ctx context.Context, channels func() [32]float64) {
	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		Parity:   serial.EvenParity,
		StopBits: serial.TwoStopBits,
	}

	port, err := serial.Open(o.port, mode)
	if err != nil {
		log.Printf("sbus: open %s: %v", o.port, err)
		return
	}
	defer port.Close()
	log.Printf("sbus: started on %s at %d baud", o.port, baudRate)

	frame := make([]byte, 25)
	ticker := time.NewTicker(time.Second / tickRate)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ch := channels()
			buildFrame(frame, ch)
			if _, err := port.Write(frame); err != nil {
				log.Printf("sbus: write error: %v", err)
				return
			}
		}
	}
}

// buildFrame encodes 16 channels into a 25-byte SBUS frame.
func buildFrame(buf []byte, ch [32]float64) {
	buf[0] = startByte

	// Scale [-1.0, 1.0] → SBUS range [172, 1811]
	vals := [numChannels]uint16{}
	for i := 0; i < numChannels; i++ {
		v := ch[i]
		if v < -1 {
			v = -1
		}
		if v > 1 {
			v = 1
		}
		vals[i] = uint16((v+1.0)/2.0*float64(1811-172) + 172)
	}

	packChannels(buf[1:23], vals)

	buf[23] = 0x00 // flags: no failsafe, no frame lost
	buf[24] = endByte
}

// packChannels packs 16 × 11-bit values into 22 bytes, LSB first.
func packChannels(dst []byte, ch [numChannels]uint16) {
	for i := range dst {
		dst[i] = 0
	}
	for i, v := range ch {
		bitPos := i * 11
		bytePos := bitPos / 8
		bitOff := bitPos % 8
		dst[bytePos] |= byte(v << bitOff)
		if bitOff > 0 {
			dst[bytePos+1] |= byte(v >> (8 - bitOff))
		}
		if bitOff > 5 {
			dst[bytePos+2] |= byte(v >> (16 - bitOff))
		}
	}
}
