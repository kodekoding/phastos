package generator

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/v2/go/helper"
)

type PDFs interface {
	SetTemplate(templatePath string, data interface{}) PDFs
	SetFooterHTMLTemplate(footerHTMLPath string) PDFs
	SetFileName(fileName *string) PDFs
	AddCustomFunction(aliasName string, function any) PDFs
	Generate() error
	Error() error
}

type PDF struct {
	generator      *wkhtmltopdf.PDFGenerator
	tmpl           *template.Template
	funcMap        template.FuncMap
	data           any
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

const templateName = "generated_pdf"

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

	tmpl := template.New(templateName)
	return &PDF{generator: generator, tmpl: tmpl}, nil
}

func (c *PDF) SetTemplate(templatePath string, data interface{}) PDFs {
	if c.err != nil {
		return c
	}
	var err error
	if c.funcMap != nil {
		c.tmpl.Funcs(c.funcMap)
	}
	c.tmpl, err = c.tmpl.Parse(templatePath)
	if err != nil {
		c.err = errors.Wrap(err, "phastos.generator.pdf.SetTemplate.ParseFile")
		return c
	}
	c.data = data
	return c
}

func (c *PDF) AddCustomFunction(aliasName string, function any) PDFs {
	if c.err != nil {
		return c
	}
	if c.funcMap == nil {
		c.funcMap = make(template.FuncMap)
	}
	c.funcMap[aliasName] = function
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
	if c.tmpl == nil {
		return errors.New("PDF Template is required")
	}

	buff := new(bytes.Buffer)
	if err := c.tmpl.ExecuteTemplate(buff, templateName, c.data); err != nil {
		return errors.Wrap(err, "phastos.generator.pdf.Generate.ExecuteTemplate")
	}

	contentString := buff.String()
	pageContent := wkhtmltopdf.NewPageReader(strings.NewReader(contentString))
	pageContent.EnableLocalFileAccess.Set(true)
	if c.footerHTMLPath != "" {
		pageContent.FooterHTML.Set(c.footerHTMLPath)
		pageContent.FooterSpacing.Set(-20.0)
	}

	c.generator.AddPage(pageContent)
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
