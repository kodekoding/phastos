package generator

import (
	"crypto/md5"
	"fmt"

	"github.com/yeqown/go-qrcode"

	"github.com/kodekoding/phastos/v2/go/helper"
	"github.com/pkg/errors"
)

type (
	QRs interface {
		SetLogoImg(logoPath string) QRs
		SetFileName(fileName *string) QRs
		Generate() error
	}
	QR struct {
		content  string
		obj      *qrcode.QRCode
		logoPath string
		fileName string
	}
)

func NewQR(content string) (QRs, error) {
	qrc, err := qrcode.New(content)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.go.generator.qr.NewQR")
	}
	return &QR{
		obj: qrc,
	}, nil
}

func (q *QR) SetLogoImg(logoPath string) QRs {
	q.logoPath = logoPath
	return q
}

func (q *QR) SetFileName(fileName *string) QRs {
	tmpFolderPath, _ := helper.GetTmpFolderPath()
	generatedName := fmt.Sprintf("%s/qr/%x.jpeg", tmpFolderPath, md5.Sum([]byte(*fileName)))
	helper.CheckFolder(generatedName)
	*fileName = generatedName
	q.fileName = generatedName
	return q
}

func (q *QR) Generate() error {

	if q.fileName == "" {
		return errors.New("fileName cannot be empty")
	}

	if err := q.obj.Save(q.fileName); err != nil {
		return errors.Wrap(err, "phastos.generator.qr.SaveObj")
	}
	return nil
}
