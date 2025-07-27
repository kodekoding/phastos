package generator

import (
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"strings"

	"github.com/fogleman/gg"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/v2/go/helper"
	plog "github.com/kodekoding/phastos/v2/go/log"
)

type (
	Banners interface {
		AddImageLayer(img *ImageLayer) Banners
		AddLabel(labelText *Label) Banners
		Generate() Banners
		Image() image.Image
		Save(destPath string) error
	}

	Options func(*Banner)
	Banner  struct {
		imgLayer []*ImageLayer
		label    []*Label
		BgProperty
		img image.Image
	}
	//ImageLayer is a struct
	ImageLayer struct {
		Image image.Image
		XPos  int
		YPos  int
	}

	//Label is a struct
	Label struct {
		Text        string
		FontPath    string
		FontType    string
		Size        float64
		Color       color.Color
		DPI         float64
		Spacing     float64
		XPos        int
		YPos        int
		RightMargin float64
	}

	//BgProperty is background property struct
	BgProperty struct {
		Width   int
		Height  int
		BgColor color.Color
	}
)

// NewBanner - by default will be w: 1200, h: 400, bgColor: white
func NewBanner(opts ...Options) *Banner {
	defaultBgColor, _ := helper.ParseHexColor("#fff")
	bannerObj := &Banner{
		BgProperty: BgProperty{
			Width:   1200,
			Height:  400,
			BgColor: defaultBgColor,
		},
	}
	for _, opt := range opts {
		opt(bannerObj)
	}
	return bannerObj
}

func WithWidth(width int) Options {
	return func(banner *Banner) {
		banner.Width = width
	}
}

func WithHeight(height int) Options {
	return func(banner *Banner) {
		banner.Height = height
	}
}

func WithBackgroudColor(hexColor string) Options {
	log := plog.Get()
	return func(banner *Banner) {
		rgba, err := helper.ParseHexColor(hexColor)
		if err != nil {
			log.Err(err).Msg("got error when parse hex string")
		}
		banner.BgColor = rgba
	}
}

func (b *Banner) AddImageLayer(img *ImageLayer) Banners {
	b.imgLayer = append(b.imgLayer, img)
	return b
}

func (b *Banner) AddLabel(labelText *Label) Banners {
	b.label = append(b.label, labelText)
	return b
}

func (b *Banner) Image() image.Image {
	return b.img
}

func (b *Banner) Generate() Banners {
	log := plog.Get()
	//create image's background
	dc := gg.NewContext(b.Width, b.Height)
	bgImg := image.NewRGBA(image.Rect(0, 0, b.Width, b.Height))

	//set the background color
	draw.Draw(bgImg, bgImg.Bounds(), &image.Uniform{b.BgColor}, image.Point{}, draw.Src)
	dc.DrawImage(bgImg, 0, 0)
	//looping image layer, higher array index = upper layer
	for _, img := range b.imgLayer {
		dc.DrawImage(img.Image, img.XPos, img.YPos)
	}

	for _, label := range b.label {
		if err := dc.LoadFontFace(label.FontPath, label.Size); err != nil {
			log.Err(err).Msg("got error when load font face")
			return nil
		}

		x := float64(label.XPos)
		y := float64(label.YPos)
		maxWidth := float64(dc.Width()) - label.RightMargin - label.RightMargin
		dc.DrawStringWrapped(label.Text, x+1, y+1, 0, 0, maxWidth, label.Spacing, gg.AlignLeft)
		dc.SetColor(label.Color)
		dc.DrawStringWrapped(label.Text, x, y, 0, 0, maxWidth, label.Spacing, gg.AlignLeft)
	}

	b.img = dc.Image()
	return b
}

func (b *Banner) Save(destPath string) error {
	newFile, err := os.Create(destPath)
	if err != nil {
		return errors.Wrap(err, "phastos.go.generator.banner.Generate.CreateNewFile")
	}
	defer newFile.Close()

	splitSavePath := strings.Split(destPath, ".")
	saveFileExt := splitSavePath[len(splitSavePath)-1]
	switch saveFileExt {
	case "png":
		if err = png.Encode(newFile, b.img); err != nil {
			return errors.Wrap(err, "phastos.go.generator.banner.Generate.EncodeToPNG")
		}
	case "jpg", "jpeg":
		var opt jpeg.Options
		opt.Quality = 80

		if err = jpeg.Encode(newFile, b.img, &opt); err != nil {
			return errors.Wrap(err, "phastos.go.generator.banner.Generate.EncodeToJPEG")
		}
	case "gif":
		var opt gif.Options

		if err = gif.Encode(newFile, b.img, &opt); err != nil {
			return errors.Wrap(err, "phastos.go.generator.banner.Generate.EncodeToGif")
		}
	default:
		return errors.Wrap(errors.New("file extensions isn't support yet"), "phastos.go.generator.banner.Generate.NewFileExtensionCheck")
	}

	return nil
}
