package engine

import (
	"context"
	"encoding/binary"
	"io"
	"math"
)

const (
	tapRate     = 4000
	levelWindow = tapRate / 20 // 50 ms in frames
)

func newEvent(p Phase) Event { return Event{Phase: p, Column: -1} }

func abs16(v int16) int32 {
	x := int32(v)
	if x < 0 {
		return -x
	}
	return x
}

// fold reads interleaved stereo int16le PCM at 4 kHz and reduces it to
// waveform columns and level events. send returns false when the consumer is
// gone, which aborts the fold with an error.
func fold(r io.Reader, duration float64, columns int, send func(Event) bool) ([]float32, error) {
	if columns <= 0 {
		columns = 192
	}
	amps := make([]float32, columns)
	total := int64(math.Round(duration * tapRate))

	var (
		frame        int64
		col          int
		colPeak      int32
		winL, winR   int32
		winN         int
		carry        [4]byte
		carryN       int
		buf          = make([]byte, 16384)
		aborted      = false
		finishColumn = func(c int, peak int32) bool {
			a := float32(peak) / 32768
			amps[c] = a
			ev := newEvent(PhaseDecode)
			ev.Column, ev.Amp = c, a
			return send(ev)
		}
	)

	handle := func(l, rr int16) bool {
		al, ar := abs16(l), abs16(rr)
		c := 0
		if total > 0 {
			c = int(frame * int64(columns) / total)
		}
		if c >= columns {
			c = columns - 1
		}
		for col < c {
			if !finishColumn(col, colPeak) {
				return false
			}
			col++
			colPeak = 0
		}
		if al > colPeak {
			colPeak = al
		}
		if ar > colPeak {
			colPeak = ar
		}
		if al > winL {
			winL = al
		}
		if ar > winR {
			winR = ar
		}
		winN++
		frame++
		if winN == levelWindow {
			ev := newEvent(PhaseDecode)
			ev.LevelL, ev.LevelR = float32(winL)/32768, float32(winR)/32768
			winL, winR, winN = 0, 0, 0
			return send(ev)
		}
		return true
	}

	for !aborted {
		n, err := r.Read(buf)
		data := buf[:n]
		for len(data) > 0 && !aborted {
			if carryN > 0 {
				k := copy(carry[carryN:], data)
				carryN += k
				data = data[k:]
				if carryN < 4 {
					break
				}
				carryN = 0
				aborted = !handle(int16(binary.LittleEndian.Uint16(carry[0:])), int16(binary.LittleEndian.Uint16(carry[2:])))
				continue
			}
			if len(data) < 4 {
				carryN = copy(carry[:], data)
				break
			}
			aborted = !handle(int16(binary.LittleEndian.Uint16(data[0:])), int16(binary.LittleEndian.Uint16(data[2:])))
			data = data[4:]
		}
		if err != nil {
			if err != io.EOF && !aborted {
				return amps, err
			}
			break
		}
	}
	if aborted {
		return amps, context.Canceled
	}
	if frame > 0 {
		if !finishColumn(col, colPeak) {
			return amps, context.Canceled
		}
	}
	return amps, nil
}

// spark reduces amps to 16 buckets quantized to 0..3.
func spark(amps []float32) [16]uint8 {
	var out [16]uint8
	n := len(amps)
	for i := 0; i < 16; i++ {
		lo, hi := i*n/16, (i+1)*n/16
		var m float32
		for _, v := range amps[lo:hi] {
			if v > m {
				m = v
			}
		}
		q := math.Ceil(float64(m) * 3)
		if q > 3 {
			q = 3
		}
		if q < 0 {
			q = 0
		}
		out[i] = uint8(q)
	}
	return out
}
