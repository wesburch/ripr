package tui

import "ripr/internal/tui/draw"

// Palette is a theme: colors by role. Two accents with fixed meaning:
// ember is anything happening now, wisp is anything finished.
type Palette struct {
	Name   string
	ground string // painted behind every cell when non-empty
	fg     string // bone
	dim    string // ash
	dim2   string // darker ash
	line   string
	ember  string
	wisp   string
	gold   string
	hood   string
	void   string
	eye    string
	glow   string
	button string // text on the ember block
	thumb  []string
}

// Theme returns the palette for a config name; unknown names get crypt.
func Theme(name string) *Palette {
	switch name {
	case "bone":
		return &Palette{
			Name: "bone", ground: "#efece6", fg: "#17151c", dim: "#6f6a7c", dim2: "#a7a2b2", line: "#cfcac3",
			ember: "#d9482a", wisp: "#1f9c7c", gold: "#a8760a", hood: "#4d4562", void: "#c9c3d3", eye: "#d9482a", glow: "#ff7a5c", button: "#fff4ef",
			thumb: []string{"#d8d2e0", "#b6a7cf", "#c98aa4", "#e07a5f", "#f0c9a3"},
		}
	case "mono":
		return &Palette{
			Name: "mono", fg: "#e6e6e6", dim: "#8a8a8a", dim2: "#5a5a5a", line: "#3a3a3a",
			ember: "#ffffff", wisp: "#d0d0d0", gold: "#bbbbbb", hood: "#6a6a6a", void: "#161616", eye: "#ffffff", glow: "#ffffff", button: "#101010",
			thumb: []string{"#202020", "#3a3a3a", "#5a5a5a", "#8a8a8a", "#c0c0c0"},
		}
	default:
		return &Palette{
			Name: "crypt", fg: "#e9e3d5", dim: "#6b667a", dim2: "#4a4558", line: "#2a2634",
			ember: "#ff5d3a", wisp: "#6fe3c1", gold: "#f2c14e", hood: "#4d4562", void: "#15121d", eye: "#ff5d3a", glow: "#ffd9cc", button: "#0b0a0f",
			thumb: []string{"#1b1a2b", "#3d2e5c", "#8a4b6b", "#e07a5f", "#f4d1ae"},
		}
	}
}

// Ground implements draw.Palette.
func (p *Palette) Ground() string { return p.ground }

// Resolve implements draw.Palette.
func (p *Palette) Resolve(s draw.Style) (fg, bg string, bold, ul bool) {
	switch s {
	case draw.Plain:
		return p.fg, "", false, false
	case draw.Dim:
		return p.dim, "", false, false
	case draw.Dim2:
		return p.dim2, "", false, false
	case draw.Line:
		return p.line, "", false, false
	case draw.Bold:
		return p.fg, "", true, false
	case draw.BoldU:
		return p.fg, "", true, true
	case draw.Ember:
		return p.ember, "", false, false
	case draw.EmberBold:
		return p.ember, "", true, false
	case draw.EmberBoldU:
		return p.ember, "", true, true
	case draw.EmberU:
		return p.ember, "", false, true
	case draw.Wisp:
		return p.wisp, "", false, false
	case draw.WispBold:
		return p.wisp, "", true, false
	case draw.Gold:
		return p.gold, "", false, false
	case draw.Button:
		return p.button, p.ember, true, false
	case draw.Cursor:
		return p.button, p.ember, false, false
	case draw.Wave:
		return p.ember, "", false, false
	case draw.WaveWisp:
		return p.wisp, "", false, false
	case draw.WaveDim:
		return p.dim2, "", false, false
	case draw.TearEdge:
		return p.ember, "", false, false
	}
	return p.fg, "", false, false
}

// Cowl returns the mascot's colors.
func (p *Palette) Cowl() draw.CowlColors {
	return draw.CowlColors{Hood: p.hood, Void: p.void, Eye: p.eye, Glow: p.glow}
}
