// Package ppm outputs a PPM waveform as PCM audio via aplay.
// Standard PPM: 20ms frame, 300µs separator pulses, channel pulses 1000–2000µs.
// The audio output should be connected to the trainer port of an RC transmitter
// via a 3.5mm audio cable.
package ppm

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"os/exec"
	"time"
)

const (
	sampleRate  = 48000
	numChannels = 8
	frameUs     = 20000 // 20ms frame
	sepUs       = 300   // separator pulse width µs
	minPulseUs  = 1000  // min channel pulse µs
	maxPulseUs  = 2000  // max channel pulse µs
	hiLevel     = 32767
	loLevel     = -32768
)

// samplesFor returns the number of PCM samples for a given microsecond duration.
func samplesFor(us int) int {
	return (us * sampleRate) / 1_000_000
}

type Output struct {
	device string
}

func New(device string) *Output {
	return &Output{device: device}
}

// Run starts aplay and pipes PCM frames until ctx is cancelled.
func (o *Output) Run(ctx context.Context, channels func() [32]float64) {
	args := []string{
		"-f", "S16_LE",
		"-r", "48000",
		"-c", "1",
	}
	if o.device != "" && o.device != "default" {
		args = append(args, "-D", o.device)
	}

	cmd := exec.CommandContext(ctx, "aplay", args...)
	pipe, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("ppm: stdin pipe: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("ppm: start aplay: %v", err)
		return
	}
	defer cmd.Wait()
	log.Printf("ppm: started aplay on device %q", o.device)

	// Pre-allocate frame buffer: worst case ~960 samples × 2 bytes each
	buf := make([]byte, 0, 2048)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			pipe.Close()
			return
		case <-ticker.C:
			ch := channels()
			buf = buildPCMFrame(buf[:0], ch)
			if _, err := io.Writer(pipe).Write(buf); err != nil {
				log.Printf("ppm: write error: %v", err)
				return
			}
		}
	}
}

// buildPCMFrame generates one 20ms PPM frame as S16_LE mono PCM samples.
// PPM: for each channel — low separator pulse (300µs) + high channel pulse (1000–2000µs).
// Remaining frame time filled with sync low pulse.
func buildPCMFrame(buf []byte, ch [32]float64) []byte {
	totalSamples := samplesFor(frameUs)
	usedSamples := 0

	writeSamples := func(n int, level int16) {
		b := [2]byte{}
		binary.LittleEndian.PutUint16(b[:], uint16(level))
		for i := 0; i < n; i++ {
			buf = append(buf, b[0], b[1])
		}
		usedSamples += n
	}

	for i := 0; i < numChannels; i++ {
		v := ch[i]
		if v < -1 {
			v = -1
		}
		if v > 1 {
			v = 1
		}
		pulseUs := int(minPulseUs + int((v+1.0)/2.0*float64(maxPulseUs-minPulseUs)))

		writeSamples(samplesFor(sepUs), int16(loLevel))
		writeSamples(samplesFor(pulseUs), int16(hiLevel))
	}

	// Final sync separator
	writeSamples(samplesFor(sepUs), int16(loLevel))

	// Pad remaining frame with sync pulse (low)
	remaining := totalSamples - usedSamples
	if remaining > 0 {
		writeSamples(remaining, int16(loLevel))
	}

	return buf
}
