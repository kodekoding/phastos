package generator

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	wkhtmltopdf "github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/go/helper"
)

type PDFs interface {
	SetTemplate(templatePath string, data interface{}) PDFs
	SetFooterHTMLTemplate(footerHTMLPath string) PDFs
	SetFileName(fileName *string) PDFs
	Generate() error
	Error() error
}

type PDF struct {
	generator      *wkhtmltopdf.PDFGenerator
	contents       *wkhtmltopdf.PageReader
	footerHTMLPath string
	fileName       string
	err            error
}

type ConverterOptions struct {
	PageSize     string
	MarginBottom uint
	MarginTop    uint
	MarginLeft   uint
	MarginRight  uint
}

func NewPDF(options ...*ConverterOptions) (*PDF, error) {
	generator, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		return nil, errors.Wrap(err, "phastos.generator.pdf.New")
	}

	// set default options
	var pageSize = wkhtmltopdf.PageSizeA4
	var marginBottom, marginTop, marginLeft, marginRight uint = 10, 10, 11, 11

	if options != nil && len(options) > 0 {
		pdfOption := options[0]
		pageSize = pdfOption.PageSize
		marginBottom = pdfOption.MarginBottom
		marginRight = pdfOption.MarginRight
		marginLeft = pdfOption.MarginLeft
		marginTop = pdfOption.MarginTop
	}

	generator.PageSize.Set(pageSize)
	generator.MarginRight.Set(marginRight)
	generator.MarginLeft.Set(marginLeft)
	generator.MarginTop.Set(marginTop)
	generator.MarginBottom.Set(marginBottom)

	return &PDF{generator: generator}, nil
}

func (c *PDF) SetTemplate(templatePath string, data interface{}) PDFs {
	if c.err != nil {
		return c
	}
	templ, err := template.ParseFiles(templatePath)
	if err != nil {
		c.err = errors.Wrap(err, "phastos.generator.pdf.SetTemplate.ParseFile")
		return c
	}

	buff := new(bytes.Buffer)
	if err = templ.Execute(buff, data); err != nil {
		c.err = errors.Wrap(err, "phastos.generator.pdf.SetTemplate.ExecuteTemplate")
		return c
	}

	contentString := buff.String()
	c.contents = wkhtmltopdf.NewPageReader(strings.NewReader(contentString))
	return c
}

func (c *PDF) SetFileName(fileName *string) PDFs {
	tmpFolderPath, _ := helper.GetTmpFolderPath()
	*fileName = fmt.Sprintf("%s/pdf/%s", tmpFolderPath, *fileName)
	helper.CheckFolder(*fileName)
	c.fileName = *fileName
	return c
}

func (c *PDF) Generate() error {
	if c.Error() != nil {
		return c.Error()
	}
	if c.footerHTMLPath != "" {
		c.contents.FooterHTML.Set(c.footerHTMLPath)
		c.contents.FooterSpacing.Set(-20.0)
	}

	c.generator.AddPage(c.contents)
	if err := c.generator.Create(); err != nil {
		return errors.Wrap(err, "phastos.generator.pdf.Generate.Create")
	}

	if err := c.generator.WriteFile(c.fileName); err != nil {
		return errors.Wrap(err, "phastos.generator.pdf.Generate.Create")
	}

	return nil
}

func (c *PDF) SetFooterHTMLTemplate(footerHTMLPath string) PDFs {
	c.footerHTMLPath = footerHTMLPath
	return c
}

func (c *PDF) Error() error {
	return c.err
}
