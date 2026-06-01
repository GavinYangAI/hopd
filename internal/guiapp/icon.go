package guiapp

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"fyne.io/fyne/v2"
	"github.com/GavinYangAI/hopd/internal/gui"
)

const iconSize = 44 // rendered at 2x so the menu-bar glyph stays crisp

// logoNeutral is the muted blue-grey of the logo's arc and the two upstream
// dots (matches Icon.png). The destination dot takes the live state colour.
var logoNeutral = color.RGBA{0x8f, 0x97, 0xa6, 0xff}

func overallColor(o gui.Overall) color.RGBA {
	switch o {
	case gui.OverallAllUp:
		return color.RGBA{0x35, 0xd0, 0x6a, 0xff} // green (up)
	case gui.OverallProblem:
		return color.RGBA{0xff, 0x5e, 0x51, 0xff} // red (error)
	case gui.OverallBusy:
		return color.RGBA{0xff, 0xa7, 0x24, 0xff} // amber (starting/retrying/auth)
	default:
		return color.RGBA{0x86, 0x8d, 0x9c, 0xff} // neutral / disconnected grey
	}
}

// iconFor renders the "hop" glyph (an arc hopping between two endpoints) tinted
// with the overall-state colour, as a PNG resource for the menu-bar tray.
func iconFor(o gui.Overall) fyne.Resource {
	c := overallColor(o)
	img := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))
	drawHopGlyph(img, c)
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	name := "hopd-" + overallName(o) + ".png"
	return fyne.NewStaticResource(name, buf.Bytes())
}

// drawHopGlyph paints the hopd "hop" logo: an arc rising from a low-left dot to
// a top apex dot and down to a low-right destination dot. The arc and the two
// upstream dots are neutral grey; the destination dot takes the overall-state
// colour (green/amber/red/grey) — mirroring Icon.png, whose endpoint is green
// when connected. Supersampled for clean edges at small sizes.
func drawHopGlyph(img *image.RGBA, dest color.RGBA) {
	const ss = 4 // supersample factor
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	fw, fh := float64(w), float64(h)

	x0, x1 := 0.15*fw, 0.85*fw // left / right endpoint x
	midX := (x0 + x1) / 2
	yEnd := 0.64 * fh // endpoints sit low
	yTop := 0.30 * fh // apex (top hop dot)
	dotR := 0.155 * fw
	lineHalf := 0.06 * fw // half stroke width

	span := (x1 - x0) / 2
	arcY := func(x float64) float64 {
		t := (x - midX) / span // -1..1 across the arc
		return yTop + (yEnd-yTop)*t*t
	}

	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			var nHits, dHits int // neutral vs destination coverage
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					fx := float64(px) + (float64(sx)+0.5)/ss
					fy := float64(py) + (float64(sy)+0.5)/ss
					switch {
					case inDot(fx, fy, x1, yEnd, dotR):
						dHits++ // destination (coloured) dot
					case inDot(fx, fy, x0, yEnd, dotR), inDot(fx, fy, midX, yTop, dotR):
						nHits++ // upstream dots
					case fx >= x0 && fx <= x1:
						if diff := fy - arcY(fx); diff > -lineHalf && diff < lineHalf {
							nHits++ // arc stroke
						}
					}
				}
			}
			total := ss * ss
			if dHits > 0 {
				img.Set(px, py, withAlpha(dest, dHits, total))
			} else if nHits > 0 {
				img.Set(px, py, withAlpha(logoNeutral, nHits, total))
			}
		}
	}
}

func withAlpha(c color.RGBA, hits, total int) color.RGBA {
	return color.RGBA{c.R, c.G, c.B, uint8(uint32(c.A) * uint32(hits) / uint32(total))}
}

func inDot(x, y, cx, cy, r float64) bool {
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

func overallName(o gui.Overall) string {
	switch o {
	case gui.OverallAllUp:
		return "up"
	case gui.OverallProblem:
		return "problem"
	case gui.OverallBusy:
		return "busy"
	case gui.OverallDisconnected:
		return "disconnected"
	default:
		return "neutral"
	}
}
