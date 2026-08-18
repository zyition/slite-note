// Command genicon draws the slite sticky-note icon as PNG files:
//   build/appicon.png  (256x256, used by wails3 generate icons for the exe)
//   icons/tray.png     (32x32, embedded into the Go binary for the system tray)
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

const (
	yellow     = 0xFFD64F
	yellowDark = 0xE8B300
	lineGray   = 0xB89A2E
)

type rgba = color.RGBA

func rgb(v int) rgba {
	return rgba{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

func main() {
	appIcon := drawIcon(256)
	writePNG("build/appicon.png", appIcon)
	tray := drawIcon(32)
	writePNG("icons/tray.png", tray)
	fmt.Println("icons written: build/appicon.png, icons/tray.png")
}

func drawIcon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	m := size / 32 // scale unit: 1 unit = size/32 px

	// rounded rect helper: fill rows between y0..y1, x inset by corner radius on edge rows
	radius := m * 6
	fillRoundedRect(img, m*2, m*2, size-m*2, size-m*2, radius, rgb(yellow))

	// folded corner (bottom-right triangle)
	fold := m * 12
	for dy := 0; dy < fold; dy++ {
		for dx := 0; dx < fold-dy; dx++ {
			img.Set(size-m*2-dx, size-m*2-dy, rgb(yellowDark))
		}
	}
	// fold hypotenuse edge
	for i := 0; i < fold; i++ {
		img.Set(size-m*2-fold+i, size-m*2-i, rgb(0x9A7B00))
	}

	// text lines (three "content" strokes)
	lineH := m * 2
	startX := m * 6
	startY := m * 7
	widths := []int{m * 20, m * 16, m * 12}
	for i, w := range widths {
		fillRect(img, startX, startY+i*(lineH+m*2), startX+w, startY+i*(lineH+m*2)+lineH, rgb(lineGray))
	}

	// outline
	outlineRoundedRect(img, m*2, m*2, size-m*2, size-m*2, radius, rgb(0x9A7B00))
	return img
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c rgba) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.Set(x, y, c)
		}
	}
}

func fillRoundedRect(img *image.RGBA, x0, y0, x1, y1, r int, c rgba) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			// corner rounding: skip pixels outside the rounded shape
			if x < x0+r && y < y0+r {
				if (x-(x0+r))*(x-(x0+r))+(y-(y0+r))*(y-(y0+r)) > r*r {
					continue
				}
			}
			if x > x1-r && y < y0+r {
				if (x-(x1-r))*(x-(x1-r))+(y-(y0+r))*(y-(y0+r)) > r*r {
					continue
				}
			}
			if x < x0+r && y > y1-r {
				if (x-(x0+r))*(x-(x0+r))+(y-(y1-r))*(y-(y1-r)) > r*r {
					continue
				}
			}
			if x > x1-r && y > y1-r {
				if (x-(x1-r))*(x-(x1-r))+(y-(y1-r))*(y-(y1-r)) > r*r {
					continue
				}
			}
			img.Set(x, y, c)
		}
	}
}

func outlineRoundedRect(img *image.RGBA, x0, y0, x1, y1, r int, c rgba) {
	// draw a 1px outline by painting the rounded rect border: top/bottom rows and left/right cols
	// (simple approach: draw the fill outline via edge rows)
	outline := image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
	fillRoundedRect(outline, x0, y0, x1, y1, r, c)
	// subtract the inner rect to leave a 1px ring
	inner := image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
	fillRoundedRect(inner, x0+1, y0+1, x1-1, y1-1, max(r-1, 0), c)
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			oc := outline.RGBAAt(x, y)
			ic := inner.RGBAAt(x, y)
			if oc.A > 0 && ic.A == 0 {
				img.Set(x, y, c)
			}
		}
	}
}

func writePNG(path string, img image.Image) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}
