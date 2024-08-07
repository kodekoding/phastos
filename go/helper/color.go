package helper

import (
	"fmt"
	"image"
	"image/color"

	"github.com/rs/zerolog/log"
)

func ParseHexColor(s string) (c color.RGBA, err error) {
	c.A = 0xff
	switch len(s) {
	case 7:
		_, err = fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	case 4:
		_, err = fmt.Sscanf(s, "#%1x%1x%1x", &c.R, &c.G, &c.B)
		// Double the hex digits:
		c.R *= 17
		c.G *= 17
		c.B *= 17
	default:
		err = fmt.Errorf("invalid length, must be 7 or 4")

	}
	return
}

func GetColorUniform(hexColor string) *image.Uniform {
	colorRGBA, err := ParseHexColor(hexColor)
	if err != nil {
		log.Err(err).Msg("got error when parse Hex: ")
	}
	return &image.Uniform{C: colorRGBA}
}
