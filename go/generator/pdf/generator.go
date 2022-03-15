package pdf

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/kodekoding/phastos/go/helper"

	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
)

type (
	Generators interface {
		ParseTemplate(templateName string, data interface{}) (result string, err error)
		GeneratePDF(fileName string, value string, footerHTMLPath string) (encodedPDF string, fileNameResult string, err error)
	}

	Generator struct{}
)

func NewGenerator() *Generator {
	return &Generator{}
}

func (f *Generator) ParseTemplate(templateName string, data interface{}) (result string, err error) {
	t, err := template.ParseFiles(templateName)
	if err != nil {
		return result, err
	}
	buf := new(bytes.Buffer)
	if err = t.Execute(buf, data); err != nil {
		return result, err
	}

	return buf.String(), nil
}

func (f *Generator) GeneratePDF(fileName, value string, footerHTMLPath string) (encodedPDF string, fileNameResult string, err error) {
	// Initialize library.
	pdfg, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		return
	}

	tmpFolderPath, _ := helper.GetTmpFolderPath()

	generatedFileName := fmt.Sprintf("%s/%s", tmpFolderPath, fileName)

	pdfg.PageSize.Set(wkhtmltopdf.PageSizeA4)
	pdfg.MarginBottom.Set(10)
	pdfg.MarginTop.Set(10)
	pdfg.MarginLeft.Set(11)
	pdfg.MarginRight.Set(11)

	page := wkhtmltopdf.NewPageReader(strings.NewReader(value))

	if footerHTMLPath != "" {
		page.FooterHTML.Set(footerHTMLPath)
		page.FooterSpacing.Set(-20.0)
	}

	pdfg.AddPage(page)
	err = pdfg.Create()
	if err != nil {
		return
	}
	err = pdfg.WriteFile(generatedFileName)
	if err != nil {
		return
	}

	return
}
