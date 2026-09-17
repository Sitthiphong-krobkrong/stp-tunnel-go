// Command genicon renders the STP Tunnel app icon to icon.png.
//
// The icon is drawn purely with the Go standard library (no external
// tooling), so it can be regenerated on any machine with:
//
//	go run ./tools/genicon
//
// Design: a rounded dark tile matching the app's theme, with a blue
// "tunnel" arc connecting a local endpoint (left) to a remote endpoint
// (right). The remote dot is green to echo the RUNNING state colour.
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const size = 512

// Palette mirrors theme.go so the icon sits naturally alongside the UI.
var (
	bgTop    = color.NRGBA{R: 0x22, G: 0x2a, B: 0x3a, A: 0xff} // subtle top tint
	bgBottom = color.NRGBA{R: 0x16, G: 0x17, B: 0x19, A: 0xff} // near-black bottom
	accent   = color.NRGBA{R: 0x3d, G: 0x7f, B: 0xd6, A: 0xff} // primary blue
	accentHi = color.NRGBA{R: 0x5f, G: 0x9c, B: 0xef, A: 0xff} // lighter blue
	success  = color.NRGBA{R: 0x3a, G: 0xa6, B: 0x4c, A: 0xff} // running green
	fg       = color.NRGBA{R: 0xe8, G: 0xe9, B: 0xea, A: 0xff} // foreground
)

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	// Rounded-square background with a vertical gradient, anti-aliased edges.
	radius := float64(size) * 0.22
	for y := 0; y < size; y++ {
		t := float64(y) / float64(size-1)
		base := lerp(bgTop, bgBottom, t)
		for x := 0; x < size; x++ {
			cov := roundedRectCoverage(float64(x)+0.5, float64(y)+0.5, size, size, radius)
			if cov <= 0 {
				continue
			}
			c := base
			c.A = uint8(float64(base.A) * cov)
			img.SetNRGBA(x, y, c)
		}
	}

	cx := float64(size) / 2

	// The tunnel: a smooth arc from the local side to the remote side.
	// Endpoints sit low; the arc bows upward through the middle.
	lx, ly := float64(size)*0.24, float64(size)*0.66
	rx, ry := float64(size)*0.76, float64(size)*0.66
	ctrlX, ctrlY := cx, float64(size)*0.30-40

	// Draw the tunnel as a TAPERED stroke: thick at the local end, thinning
	// toward the remote end. The taper itself signals direction, and we stop
	// the arc short of the remote node so a clear arrowhead can cap it.
	const tEnd = 0.82 // where the stroke ends; arrowhead covers the rest
	startR := float64(size) * 0.058
	endR := float64(size) * 0.026
	for i := 0.0; i <= tEnd; i += 0.0006 {
		bx, by := quadBezier(lx, ly, ctrlX, ctrlY, rx, ry, i)
		r := startR + (endR-startR)*(i/tEnd)
		col := lerp(accent, accentHi, i/tEnd)
		stampDot(img, bx, by, r, col)
	}

	// Arrowhead capping the arc, aligned to the arc's tangent so it reads as
	// the tunnel flowing into the remote node. Placed at the end of the
	// stroke, pointing toward the remote endpoint.
	tx, ty := quadBezier(lx, ly, ctrlX, ctrlY, rx, ry, tEnd)
	dirX, dirY := bezierTangent(lx, ly, ctrlX, ctrlY, rx, ry, tEnd)
	drawArrowhead(img, tx, ty, dirX, dirY, float64(size)*0.085, accentHi)

	// Endpoint nodes: local (blue) and remote (green = reachable). Drawn
	// last so they sit cleanly on top of the arc and arrowhead.
	drawNode(img, lx, ly, float64(size)*0.072, fg, accent)
	drawNode(img, rx, ry, float64(size)*0.072, fg, success)

	out, err := os.Create("icon.png")
	if err != nil {
		panic(err)
	}
	defer out.Close()
	if err := png.Encode(out, img); err != nil {
		panic(err)
	}
}

// quadBezier returns the point at t along a quadratic Bezier.
func quadBezier(x0, y0, cx, cy, x1, y1, t float64) (float64, float64) {
	mt := 1 - t
	x := mt*mt*x0 + 2*mt*t*cx + t*t*x1
	y := mt*mt*y0 + 2*mt*t*cy + t*t*y1
	return x, y
}

// bezierTangent returns the normalised direction of travel at t.
func bezierTangent(x0, y0, cx, cy, x1, y1, t float64) (float64, float64) {
	dx := 2*(1-t)*(cx-x0) + 2*t*(x1-cx)
	dy := 2*(1-t)*(cy-y0) + 2*t*(y1-cy)
	l := math.Hypot(dx, dy)
	if l == 0 {
		return 1, 0
	}
	return dx / l, dy / l
}

// stampDot draws a soft-edged filled circle (additive over existing pixels).
func stampDot(img *image.NRGBA, cx, cy, r float64, col color.NRGBA) {
	minX := int(cx - r - 1)
	maxX := int(cx + r + 1)
	minY := int(cy - r - 1)
	maxY := int(cy + r + 1)
	for y := minY; y <= maxY; y++ {
		if y < 0 || y >= size {
			continue
		}
		for x := minX; x <= maxX; x++ {
			if x < 0 || x >= size {
				continue
			}
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			cov := 1 - smoothstep(r-1.2, r+1.2, d)
			if cov <= 0 {
				continue
			}
			blendOver(img, x, y, col, cov)
		}
	}
}

// drawNode draws a filled disc with a ring, used for tunnel endpoints.
func drawNode(img *image.NRGBA, cx, cy, r float64, ring, fill color.NRGBA) {
	stampDot(img, cx, cy, r+float64(size)*0.016, ring)
	stampDot(img, cx, cy, r, fill)
}

// drawArrowhead draws a filled triangle whose tip is at (tipX,tipY) offset
// along the (dx,dy) unit direction, so the arrow points the way the tunnel
// flows. size h is the distance from tip to base.
func drawArrowhead(img *image.NRGBA, baseX, baseY, dx, dy, h float64, col color.NRGBA) {
	// Perpendicular to the direction, for the base half-width.
	px, py := -dy, dx
	halfW := h * 0.62

	// The tip is h ahead of the given point along the direction; the base
	// spans halfW to each side at the given point.
	tipX := baseX + dx*h
	tipY := baseY + dy*h
	aX, aY := baseX+px*halfW, baseY+py*halfW
	bX, bY := baseX-px*halfW, baseY-py*halfW

	fillTriangle(img, tipX, tipY, aX, aY, bX, bY, col)
}

// fillTriangle rasterises a triangle with 1px anti-aliased edges.
func fillTriangle(img *image.NRGBA, x0, y0, x1, y1, x2, y2 float64, col color.NRGBA) {
	minX := int(math.Floor(math.Min(x0, math.Min(x1, x2)))) - 1
	maxX := int(math.Ceil(math.Max(x0, math.Max(x1, x2)))) + 1
	minY := int(math.Floor(math.Min(y0, math.Min(y1, y2)))) - 1
	maxY := int(math.Ceil(math.Max(y0, math.Max(y1, y2)))) + 1
	area := edge(x0, y0, x1, y1, x2, y2)
	if area == 0 {
		return
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			// 2x2 supersample for smooth edges.
			var cov float64
			for _, oy := range [2]float64{0.25, 0.75} {
				for _, ox := range [2]float64{0.25, 0.75} {
					sx, sy := float64(x)+ox, float64(y)+oy
					w0 := edge(x1, y1, x2, y2, sx, sy) / area
					w1 := edge(x2, y2, x0, y0, sx, sy) / area
					w2 := edge(x0, y0, x1, y1, sx, sy) / area
					if w0 >= 0 && w1 >= 0 && w2 >= 0 {
						cov += 0.25
					}
				}
			}
			if cov > 0 {
				blendOver(img, x, y, col, cov)
			}
		}
	}
}

// edge is the signed area of the triangle (a,b,p) ×2.
func edge(ax, ay, bx, by, px, py float64) float64 {
	return (bx-ax)*(py-ay) - (by-ay)*(px-ax)
}

// roundedRectCoverage returns anti-aliased coverage [0,1] for a rounded rect
// centred in a w×h box.
func roundedRectCoverage(px, py float64, w, h, r float64) float64 {
	hw, hh := w/2, h/2
	// Signed distance to a rounded rectangle centred in the box.
	dx := math.Abs(px-hw) - (hw - r)
	dy := math.Abs(py-hh) - (hh - r)
	ox := math.Max(dx, 0)
	oy := math.Max(dy, 0)
	dist := math.Min(math.Max(dx, dy), 0) + math.Hypot(ox, oy) - r
	return 1 - smoothstep(-1, 1, dist)
}

func smoothstep(edge0, edge1, x float64) float64 {
	if edge0 == edge1 {
		if x < edge0 {
			return 0
		}
		return 1
	}
	t := (x - edge0) / (edge1 - edge0)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

func lerp(a, b color.NRGBA, t float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: uint8(float64(a.A) + (float64(b.A)-float64(a.A))*t),
	}
}

// blendOver alpha-composites col (with extra coverage) over the pixel.
func blendOver(img *image.NRGBA, x, y int, col color.NRGBA, cov float64) {
	if x < 0 || x >= size || y < 0 || y >= size || cov <= 0 {
		return
	}
	if cov > 1 {
		cov = 1
	}
	dst := img.NRGBAAt(x, y)
	sa := float64(col.A) / 255 * cov
	da := float64(dst.A) / 255
	outA := sa + da*(1-sa)
	if outA <= 0 {
		img.SetNRGBA(x, y, color.NRGBA{})
		return
	}
	mix := func(s, d uint8) uint8 {
		sf := float64(s) * sa
		df := float64(d) * da * (1 - sa)
		return uint8((sf + df) / outA)
	}
	img.SetNRGBA(x, y, color.NRGBA{
		R: mix(col.R, dst.R),
		G: mix(col.G, dst.G),
		B: mix(col.B, dst.B),
		A: uint8(outA * 255),
	})
}
