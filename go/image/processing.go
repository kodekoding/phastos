package image

import (
	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	imglib "image"
	"io"
	"net/http"
	"os"
	"strings"
)

type (
	Processing interface {
		Resize(width, length uint) imglib.Image
	}

	processing struct {
		img     imglib.Image
		imgFile *os.File
	}
)

func Load(path string) (*processing, error) {
	var (
		imgFile *os.File
		err     error
	)

	defer imgFile.Close()
	if strings.Contains(path, "http") {
		// load image from URL, then download it first
		splitPath := strings.Split(path, "/")
		fileName := splitPath[len(splitPath)-1]
		imgFile, err = os.Create(fileName)
		if err != nil {
			return nil, errors.Wrap(err, "phastos.go.image.processing.Load.CreateNewFile")
		}
		res, err := http.Get(path)

		if err != nil {
			return nil, errors.Wrap(err, "phastos.go.image.processing.Load.GetImage")
		}

		defer res.Body.Close()

		_, err = io.Copy(imgFile, res.Body)

		if err != nil {
			return nil, errors.Wrap(err, "phastos.go.image.processing.Load.CopyContentToNewFile")
		}
	} else {
		imgFile, err = os.Open(path)
		if err != nil {
			return nil, errors.Wrap(err, "phastos.go.image.processing.Load.OpenFile")
		}
	}

	img, _, err := imglib.Decode(imgFile)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.go.helper.image.LoadImage.ImageDecode")
	}
	return &processing{
		imgFile: imgFile,
		img:     img,
	}, nil
}

func (p *processing) Resize(width, length uint) imglib.Image {
	return resize.Resize(width, length, p.img, resize.Bicubic)
}
