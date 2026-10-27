package main

import (
	"math"
	"unicode"

	"github.com/egoist/mygo/ui"
)

// avatar is a person's picture cropped round at size, or their initials
// on a color of their name, as Contacts and Messages draw them: both fill
// the circle, under the same hairline. ui.Avatar draws pictures at its
// own size whatever the element's, which leaves a ring or crops them.
func avatar(c *ui.Context, name string, photo *ui.Bitmap, size float32) ui.Element {
	e := ui.Box(c).Size(size, size).Radius(size / 2).Clip().Shrink(0).Center().Label(name)
	e.Children(func() {
		if photo != nil {
			ui.Image(c, photo).Fit(ui.Cover).Size(size, size)
		} else {
			ui.Text(c, initials(name)).FontWeight(600).FontSize(size * 0.4).SingleLine().TextColor(ui.RGB(255, 255, 255))
		}
		// The hairline over the edge, which keeps light pictures round on a
		// light background.
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(size/2).
			Border(0.5, ui.RGBA(0, 0, 0, 0.12)).PassThrough()
	})
	if photo == nil {
		top, bottom := nameColors(name)
		e.Gradient(top, bottom, 180)
	}
	return e
}

// nameColors is a gradient of the hue of name's FNV-1a hash, lighter at the
// top, as macOS's monograms are.
func nameColors(name string) (top, bottom ui.Color) {
	h := uint32(2166136261)
	for i := 0; i < len(name); i++ {
		h = (h ^ uint32(name[i])) * 16777619
	}
	hue := float64(h % 360)
	return hsl(hue, 0.45, 0.62), hsl(hue, 0.5, 0.48)
}

func hsl(h, s, l float64) ui.Color {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g = c, x
	case h < 120:
		r, g = x, c
	case h < 180:
		g, b = c, x
	case h < 240:
		g, b = x, c
	case h < 300:
		r, b = x, c
	default:
		r, b = c, x
	}
	return ui.RGB(uint8((r+m)*255), uint8((g+m)*255), uint8((b+m)*255))
}

// initials are the first letters of the first and last words of name.
func initials(name string) string {
	var first, last rune
	inWord := false
	for _, r := range name {
		letter := unicode.IsLetter(r) || unicode.IsDigit(r)
		if letter && !inWord {
			if first == 0 {
				first = unicode.ToUpper(r)
			} else {
				last = unicode.ToUpper(r)
			}
		}
		inWord = letter
	}
	switch {
	case first == 0:
		return ""
	case last == 0 || unicode.Is(unicode.Han, first):
		return string(first)
	}
	return string(first) + string(last)
}
