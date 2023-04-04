package image

import (
	"github.com/disintegration/imaging"
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
		Image() imglib.Image
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
		questionMarkIndex := strings.Index(fileName, "?")
		fileName = fileName[:questionMarkIndex]
		imgFile, err = os.Create(fileName)
		if err != nil {
			return nil, errors.Wrap(err, "phastos.go.image.processing.Load.CreateNewFile")
		}
		defer os.RemoveAll(fileName)
		res, err := http.Get(path)

		if err != nil {
			return nil, errors.Wrap(err, "phastos.go.image.processing.Load.GetImage")
		}

		defer res.Body.Close()

		_, err = io.Copy(imgFile, res.Body)

		if err != nil {
			return nil, errors.Wrap(err, "phastos.go.image.processing.Load.CopyContentToNewFile")
		}

		path = fileName
	}

	imgFile, err = os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.go.image.processing.Load.OpenFile")
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

func (p *processing) Resize(width, length int) imglib.Image {
	return imaging.Resize(p.img, width, length, imaging.Lanczos)
}

func (p *processing) Image() imglib.Image {
	return p.img
}
