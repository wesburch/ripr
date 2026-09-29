package draw

// TearJitter returns a per-row wobble for the tear edge, so the rip is
// ragged but coherent from row to row.
func TearJitter(rows int) []float64 {
	j := make([]float64, rows)
	v := 0.0
	for y := 0; y < rows; y++ {
		v += (hashf(float64(y)*7.7) - 0.5) * 4
		if v > 7 {
			v = 7
		}
		if v < -7 {
			v = -7
		}
		j[y] = v
	}
	return j
}

// Tear composes the transition from prev to next at progress p in 0..1.
// Left of the edge is the new frame, right of it the old one, and the
// edge itself is two ragged cells.
func Tear(prev, next *Frame, jitter []float64, p float64) *Frame {
	out := New(next.W, next.H)
	for y := 0; y < next.H; y++ {
		edge := p*float64(next.W+24) - 12 + jitter[y%len(jitter)] + float64(y)*0.15
		for x := 0; x < next.W; x++ {
			fx := float64(x)
			switch {
			case fx < edge-1:
				out.Cells[y][x] = next.Cells[y][x]
			case fx < edge:
				r := '▞'
				if y%2 == 1 {
					r = '▚'
				}
				out.Cells[y][x] = Cell{R: r, S: TearEdge}
			case fx < edge+1:
				out.Cells[y][x] = Cell{R: '░', S: Dim2}
			default:
				if y < prev.H && x < prev.W {
					out.Cells[y][x] = prev.Cells[y][x]
				}
			}
		}
	}
	return out
}
