package helper

import (
	"image"
	"os"

	"github.com/pkg/errors"
)

func LoadImage(filePath string) (image.Image, error) {
	imgFile, err := os.Open(filePath)
	defer imgFile.Close()
	if err != nil {
		return nil, errors.Wrap(err, "phastos.go.helper.image.LoadImage.OpenFile")
	}

	img, _, err := image.Decode(imgFile)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.go.helper.image.LoadImage.ImageDecode")

	}
	return img, nil
}
