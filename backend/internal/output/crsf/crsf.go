// Package crsf outputs CRSF (Crossfire/ExpressLRS) channel frames over serial.
// Protocol: 420100 baud 8N1. Frame type 0x16 carries 16 channels as 11-bit packed values.
package crsf

import (
	"context"
	"log"
	"time"

	serial "go.bug.st/serial"
)

const (
	baudRate    = 420100
	frameType   = 0x16
	syncByte    = 0xC8
	numChannels = 16
	tickRate    = 150 // Hz
)

type Output struct {
	port     string
	failsafe [numChannels]uint16
}

func New(port string, failsafe [numChannels]uint16) *Output {
	return &Output{port: port, failsafe: failsafe}
}

// Run opens the serial port and sends frames until ctx is cancelled.
func (o *Output) Run(ctx context.Context, channels func() [32]float64) {
	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(o.port, mode)
	if err != nil {
		log.Printf("crsf: open %s: %v", o.port, err)
		return
	}
	defer port.Close()
	log.Printf("crsf: started on %s at %d baud", o.port, baudRate)

	frame := make([]byte, 26) // sync(1) + len(1) + type(1) + payload(22) + crc(1)
	ticker := time.NewTicker(time.Second / tickRate)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ch := channels()
			buildFrame(frame, ch, o.failsafe)
			if _, err := port.Write(frame); err != nil {
				log.Printf("crsf: write error: %v", err)
				return
			}
		}
	}
}

// buildFrame encodes 16 channels as 11-bit packed values into a CRSF RC channel frame.
func buildFrame(buf []byte, ch [32]float64, fs [numChannels]uint16) {
	// Scale [-1.0, 1.0] → 11-bit CRSF range [172, 1811], centre 992
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

	// CRSF frame: [sync][len][type][22 bytes packed channels][crc]
	buf[0] = syncByte
	buf[1] = 24  // len = type(1) + payload(22) + crc(1)
	buf[2] = frameType

	payload := buf[3:25]
	packChannels(payload, vals)

	buf[25] = crc8(buf[2:25])
}

// packChannels packs 16 × 11-bit channel values into 22 bytes, LSB first.
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

var crc8Table [256]byte

func init() {
	const poly = 0xD5
	for i := 0; i < 256; i++ {
		crc := byte(i)
		for j := 0; j < 8; j++ {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ poly
			} else {
				crc <<= 1
			}
		}
		crc8Table[i] = crc
	}
}

func crc8(data []byte) byte {
	crc := byte(0)
	for _, b := range data {
		crc = crc8Table[crc^b]
	}
	return crc
}
