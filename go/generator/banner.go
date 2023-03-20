package generator

import (
	"github.com/golang/freetype"
	"github.com/pkg/errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"strings"
)

type (
	Banners interface {
		SetBgColor(colorParam color.Color) Banners
		AddImageLayer(img *ImageLayer) Banners
		AddLabel(labelText *Label) Banners
		Generate(savePath string) error
	}

	Banner struct {
		imgLayer []*ImageLayer
		label    []*Label
		BgProperty
	}
	//ImageLayer is a struct
	ImageLayer struct {
		Image image.Image
		XPos  int
		YPos  int
	}

	//Label is a struct
	Label struct {
		Text     string
		FontPath string
		FontType string
		Size     float64
		Color    image.Image
		DPI      float64
		Spacing  float64
		XPos     int
		YPos     int
	}

	//BgProperty is background property struct
	BgProperty struct {
		Width   int
		Length  int
		BgColor color.Color
	}
)

func NewBanner(width int, length int) *Banner {
	return &Banner{
		BgProperty: BgProperty{
			Width:  width,
			Length: length,
		},
	}
}

func (b *Banner) SetBgColor(colorParam color.Color) Banners {
	b.BgColor = colorParam
	return b
}

func (b *Banner) AddImageLayer(img *ImageLayer) Banners {
	b.imgLayer = append(b.imgLayer, img)
	return b
}

func (b *Banner) AddLabel(labelText *Label) Banners {
	b.label = append(b.label, labelText)
	return b
}

func (b *Banner) Generate(savePath string) error {
	//create image's background
	bgImg := image.NewRGBA(image.Rect(0, 0, b.Width, b.Length))

	//set the background color
	draw.Draw(bgImg, bgImg.Bounds(), &image.Uniform{b.BgColor}, image.Point{}, draw.Src)

	//looping image layer, higher array index = upper layer
	for _, img := range b.imgLayer {
		//set image offset
		offset := image.Pt(img.XPos, img.YPos)

		//combine the image
		draw.Draw(bgImg, img.Image.Bounds().Add(offset), img.Image, image.Point{}, draw.Over)
	}

	//add label(s)
	bgImg, err := b.addLabel(bgImg, b.label)
	if err != nil {
		return err
	}

	newFile, err := os.Create(savePath)
	if err != nil {
		return errors.Wrap(err, "phastos.go.generator.banner.Generate.CreateNewFile")
	}
	defer newFile.Close()

	splitSavePath := strings.Split(savePath, ".")
	saveFileExt := splitSavePath[len(splitSavePath)-1]
	switch saveFileExt {
	case "png":
		if err = png.Encode(newFile, bgImg); err != nil {
			return errors.Wrap(err, "phastos.go.generator.banner.Generate.EncodeToPNG")
		}
	case "jpg", "jpeg":
		var opt jpeg.Options
		opt.Quality = 80

		if err = jpeg.Encode(newFile, bgImg, &opt); err != nil {
			return errors.Wrap(err, "phastos.go.generator.banner.Generate.EncodeToJPEG")
		}
	default:
		return errors.Wrap(errors.New("file extensions isn't support yet"), "phastos.go.generator.banner.Generate.NewFileExtensionCheck")
	}

	return nil
}

func (b *Banner) addLabel(img *image.RGBA, labels []*Label) (*image.RGBA, error) {
	//initialize the context
	c := freetype.NewContext()

	for _, label := range labels {
		//read font data
		fontBytes, err := os.ReadFile(label.FontPath + label.FontType)
		if err != nil {
			return nil, err
		}
		f, err := freetype.ParseFont(fontBytes)
		if err != nil {
			return nil, err
		}

		//set label configuration
		c.SetDPI(label.DPI)
		c.SetFont(f)
		c.SetFontSize(label.Size)
		c.SetClip(img.Bounds())
		c.SetDst(img)
		c.SetSrc(label.Color)

		//positioning the label
		pt := freetype.Pt(label.XPos, label.YPos+int(c.PointToFixed(label.Size)>>6))

		//draw the label on image
		_, err = c.DrawString(label.Text, pt)
		if err != nil {
			log.Println(err)
			return img, nil
		}
		pt.Y += c.PointToFixed(label.Size * label.Spacing)
	}

	return img, nil
}
