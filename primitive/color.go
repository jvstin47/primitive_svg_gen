package primitive

import (
	"fmt"
	"image/color"
	"strings"
)

type Color struct {
	R, G, B, A int
}

func MakeColor(c color.Color) Color {
	r, g, b, a := c.RGBA()
	return Color{int(r / 257), int(g / 257), int(b / 257), int(a / 257)}
}

func MakeHexColor(x string) Color {
	x = strings.Trim(x, "#")
	var r, g, b, a int
	a = 255
	switch len(x) {
	case 3:
		fmt.Sscanf(x, "%1x%1x%1x", &r, &g, &b)
		r = (r << 4) | r
		g = (g << 4) | g
		b = (b << 4) | b
	case 4:
		fmt.Sscanf(x, "%1x%1x%1x%1x", &r, &g, &b, &a)
		r = (r << 4) | r
		g = (g << 4) | g
		b = (b << 4) | b
		a = (a << 4) | a
	case 6:
		fmt.Sscanf(x, "%02x%02x%02x", &r, &g, &b)
	case 8:
		fmt.Sscanf(x, "%02x%02x%02x%02x", &r, &g, &b, &a)
	}
	return Color{r, g, b, a}
}

func (c *Color) NRGBA() color.NRGBA {
	return color.NRGBA{uint8(c.R), uint8(c.G), uint8(c.B), uint8(c.A)}
}

func (c Color) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func (c Color) Luminance() float64 {
	return (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255.0
}

func LerpColor(c1, c2 Color, t float64) Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	r := int(float64(c1.R)*(1-t) + float64(c2.R)*t)
	g := int(float64(c1.G)*(1-t) + float64(c2.G)*t)
	b := int(float64(c1.B)*(1-t) + float64(c2.B)*t)
	return Color{clampInt(r, 0, 255), clampInt(g, 0, 255), clampInt(b, 0, 255), c1.A}
}

func (c Color) ApplyPalette(palette string, d1, d2 Color) Color {
	lum := c.Luminance()
	palette = strings.ToLower(strings.TrimSpace(palette))
	switch palette {
	case "grayscale", "gray", "bw":
		gray := int(lum * 255.0)
		return Color{gray, gray, gray, c.A}
	case "duotone":
		res := LerpColor(d1, d2, lum)
		res.A = c.A
		return res
	case "cyberpunk", "neon":
		c1 := MakeHexColor("#ff007f") // Neon pink
		c2 := MakeHexColor("#00f0ff") // Cyber cyan
		res := LerpColor(c1, c2, lum)
		res.A = c.A
		return res
	case "sunset":
		c1 := MakeHexColor("#3a0ca3") // Deep purple
		c2 := MakeHexColor("#ffbe0b") // Golden sunset
		res := LerpColor(c1, c2, lum)
		res.A = c.A
		return res
	case "sepia", "vintage":
		c1 := MakeHexColor("#2e1c0c")
		c2 := MakeHexColor("#faedcd")
		res := LerpColor(c1, c2, lum)
		res.A = c.A
		return res
	case "matrix", "emerald":
		c1 := MakeHexColor("#031d10")
		c2 := MakeHexColor("#39ff14")
		res := LerpColor(c1, c2, lum)
		res.A = c.A
		return res
	case "monochrome", "ocean":
		c1 := MakeHexColor("#03045e")
		c2 := MakeHexColor("#90e0ef")
		res := LerpColor(c1, c2, lum)
		res.A = c.A
		return res
	case "invert":
		return Color{255 - c.R, 255 - c.G, 255 - c.B, c.A}
	default:
		return c
	}
}
