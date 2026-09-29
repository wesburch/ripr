package draw

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
)

// Thumb is a thumbnail reduced to half-block cells: two pixels per cell,
// each a hex color.
type Thumb struct {
	W, H int // cells
	Top  [][]string
	Bot  [][]string
}

// DecodeThumb reads a JPEG or PNG and reduces it to w×h cells by box
// averaging. Aspect is not preserved on purpose; the block is a color
// impression, not a picture.
func DecodeThumb(r io.Reader, w, h int) (*Thumb, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	return ThumbFromImage(img, w, h), nil
}

// ThumbFromImage reduces an image to w×h cells.
func ThumbFromImage(img image.Image, w, h int) *Thumb {
	b := img.Bounds()
	// YouTube thumbnails carry black letterbox bars; crop 12% top and bottom.
	inset := int(float64(b.Dy()) * 0.12)
	b.Min.Y += inset
	b.Max.Y -= inset
	t := &Thumb{W: w, H: h, Top: make([][]string, h), Bot: make([][]string, h)}
	ph := h * 2
	for cy := 0; cy < h; cy++ {
		t.Top[cy] = make([]string, w)
		t.Bot[cy] = make([]string, w)
		for cx := 0; cx < w; cx++ {
			t.Top[cy][cx] = avg(img, b, cx, cy*2, w, ph)
			t.Bot[cy][cx] = avg(img, b, cx, cy*2+1, w, ph)
		}
	}
	return t
}

func avg(img image.Image, b image.Rectangle, px, py, pw, ph int) string {
	x0 := b.Min.X + px*b.Dx()/pw
	x1 := b.Min.X + (px+1)*b.Dx()/pw
	y0 := b.Min.Y + py*b.Dy()/ph
	y1 := b.Min.Y + (py+1)*b.Dy()/ph
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	var r, g, bl, n float64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			r += float64(c.R)
			g += float64(c.G)
			bl += float64(c.B)
			n++
		}
	}
	if n == 0 {
		return "#000000"
	}
	return fmt.Sprintf("#%02x%02x%02x", int(r/n), int(g/n), int(bl/n))
}

// Paint draws the thumbnail with its top-left cell at (x, y).
func (t *Thumb) Paint(f *Frame, x, y int) {
	for cy := 0; cy < t.H; cy++ {
		for cx := 0; cx < t.W; cx++ {
			f.Set(x+cx, y+cy, Cell{R: '▀', FG: t.Top[cy][cx], BG: t.Bot[cy][cx]})
		}
	}
}

// FauxThumb is the block-art stand-in used until a real thumbnail arrives,
// or when one cannot be decoded. Colors come from the theme's thumb ramp.
func FauxThumb(w, h int, ramp []string) *Thumb {
	t := &Thumb{W: w, H: h, Top: make([][]string, h), Bot: make([][]string, h)}
	pick := func(i, j int) string {
		v := 0.5 + 0.5*math.Sin(float64(i)*0.55+float64(j)*0.9) + 0.35*math.Sin(float64(i)*1.7-float64(j)*1.1) + (hashf(float64(i*9+j*3))-0.5)*0.5
		k := int(v * 2.2)
		if k < 0 {
			k = 0
		}
		if k >= len(ramp) {
			k = len(ramp) - 1
		}
		return ramp[k]
	}
	for cy := 0; cy < h; cy++ {
		t.Top[cy] = make([]string, w)
		t.Bot[cy] = make([]string, w)
		for cx := 0; cx < w; cx++ {
			t.Top[cy][cx] = pick(cx, cy*2)
			t.Bot[cy][cx] = pick(cx, cy*2+1)
		}
	}
	return t
}
