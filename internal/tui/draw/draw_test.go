package draw

import (
	"strings"
	"testing"
)

type testPal struct{}

func (testPal) Resolve(s Style) (string, string, bool, bool) {
	if s == Ember {
		return "#ff5d3a", "", false, false
	}
	return "#ffffff", "", false, false
}
func (testPal) Ground() string { return "" }

func TestBrailleBits(t *testing.T) {
	b := NewBraille(1, 1)
	b.Set(0, 0)
	if got := b.Rows()[0]; got != "⠁" {
		t.Fatalf("top-left dot = %q, want ⠁", got)
	}
	b.Clear()
	b.Set(1, 3)
	if got := b.Rows()[0]; got != "⢀" {
		t.Fatalf("bottom-right dot = %q, want ⢀", got)
	}
	b.Clear()
	b.VLine(0, 0, 3)
	b.VLine(1, 0, 3)
	if got := b.Rows()[0]; got != "⣿" {
		t.Fatalf("full cell = %q, want ⣿", got)
	}
}

func TestSpark(t *testing.T) {
	var v [16]uint8
	for i := range v {
		v[i] = uint8(i % 4)
	}
	s := Spark(v)
	if n := len([]rune(s)); n != 8 {
		t.Fatalf("spark is %d cells, want 8", n)
	}
}

func TestCowlHasExactlyTwoEyes(t *testing.T) {
	colors := CowlColors{Hood: "#111111", Void: "#000000", Eye: "#ff0000", Glow: "#ff8888"}
	for _, o := range []CowlOpts{{}, {Glow: true}, {Dip: true}} {
		f := New(12, 3)
		py := 1
		if o.Dip {
			py = 2
		}
		Rasterize(f, 0, 0, py, Cowl(o, colors))
		eyes := 0
		for _, row := range f.Cells {
			for _, c := range row {
				// an eye is the bright pixel drawn as the glyph, never as background
				if c.FG == "#ff0000" || c.FG == "#ff8888" {
					eyes++
				}
				if c.BG == "#ff0000" || c.BG == "#ff8888" {
					t.Fatalf("eye drawn as background: %+v", c)
				}
			}
		}
		if eyes != 2 {
			t.Fatalf("opts %+v: %d eye cells, want 2", o, eyes)
		}
	}
	f := New(12, 3)
	Rasterize(f, 0, 0, 1, Cowl(CowlOpts{Blink: true}, colors))
	for _, row := range f.Cells {
		for _, c := range row {
			if c.FG == "#ff0000" {
				t.Fatal("eyes visible while blinking")
			}
		}
	}
}

func TestWordmarkAndBoot(t *testing.T) {
	pts, w, h := WordmarkPixels(2)
	if len(pts) == 0 || w == 0 || h == 0 {
		t.Fatal("empty wordmark")
	}
	bp, bw, bh := BootPoints(2, 1)
	if len(bp) != len(pts) || bw != w || bh != h {
		t.Fatal("boot points do not match wordmark pixels")
	}
	b := NewBraille((bw+1)/2, (bh+3)/4)
	settled := DrawBoot(b, bp, bw, bh, 0.3)
	for _, s := range settled {
		if s {
			t.Fatal("nothing should be settled during the warm-up wave")
		}
	}
	settled = DrawBoot(b, bp, bw, bh, 10)
	for i, s := range settled {
		if !s {
			t.Fatalf("cell %d not settled at t=10", i)
		}
	}
	if !BootDone(bp, 10) || BootDone(bp, 1) {
		t.Fatal("BootDone wrong")
	}
	rows := b.Rows()
	if strings.TrimSpace(strings.Join(rows, "")) == strings.Repeat("⠀", len(rows)*b.W) {
		t.Fatal("settled wordmark drew nothing")
	}
}

func TestTearEndpoints(t *testing.T) {
	prev, next := New(20, 4), New(20, 4)
	prev.Put(0, 0, "old", Plain)
	next.Put(0, 0, "new", Plain)
	j := TearJitter(4)
	if got := Tear(prev, next, j, 0).Cells[0][0].R; got != 'o' {
		t.Fatalf("p=0 should show the old frame, got %q", got)
	}
	if got := Tear(prev, next, j, 1.2).Cells[0][0].R; got != 'n' {
		t.Fatalf("p>1 should show the new frame, got %q", got)
	}
}

func TestRenderRuns(t *testing.T) {
	f := New(6, 1)
	f.Put(0, 0, "ab", Plain)
	f.Put(2, 0, "cd", Ember)
	f.Put(4, 0, "ef", Plain)
	out := NewRenderer(testPal{}).Render(f)
	if !strings.Contains(out, "ab") || !strings.Contains(out, "cd") || !strings.Contains(out, "ef") {
		t.Fatalf("render lost text: %q", out)
	}
	if strings.Contains(out, "\n") {
		t.Fatal("single row must not end with a newline")
	}
}

func TestFauxThumbShape(t *testing.T) {
	th := FauxThumb(12, 3, []string{"#000", "#111", "#222"})
	if th.W != 12 || th.H != 3 || len(th.Top) != 3 || len(th.Top[0]) != 12 {
		t.Fatal("faux thumb has the wrong shape")
	}
	f := New(12, 3)
	th.Paint(f, 0, 0)
	if f.Cells[0][0].R != '▀' || f.Cells[0][0].BG == "" {
		t.Fatal("thumb cells must be half blocks with a background")
	}
}
