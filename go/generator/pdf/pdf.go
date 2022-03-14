package pdf

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	wkhtmltopdf "github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/go/helper"
)

type Converters interface {
	SetTemplate(templatePath string, data interface{}) Converters
	SetFooterHTMLTemplate(footerHTMLPath string) Converters
	Generate(fileName string) error
	Error() error
}

type Converter struct {
	generator      *wkhtmltopdf.PDFGenerator
	contents       *wkhtmltopdf.PageReader
	footerHTMLPath string
	err            error
}

type ConverterOptions struct {
	PageSize     string
	MarginBottom uint
	MarginTop    uint
	MarginLeft   uint
	MarginRight  uint
}

func New(options ...*ConverterOptions) (*Converter, error) {
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

	return &Converter{generator: generator}, nil
}

func (c *Converter) SetTemplate(templatePath string, data interface{}) Converters {
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

func (c *Converter) Generate(fileName string) error {
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

	tmpFolderPath, _ := helper.GetTmpFolderPath()
	generatedFileName := fmt.Sprintf("%s/%s", tmpFolderPath, fileName)
	if err := c.generator.WriteFile(generatedFileName); err != nil {
		return errors.Wrap(err, "phastos.generator.pdf.Generate.Create")
	}

	return nil
}

func (c *Converter) SetFooterHTMLTemplate(footerHTMLPath string) Converters {
	c.footerHTMLPath = footerHTMLPath
	return c
}

func (c *Converter) Error() error {
	return c.err
}
