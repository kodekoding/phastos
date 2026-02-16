package generator_test

import (
	"fmt"
	"image/color"
	"log"
	"net/http"
	"os"

	"github.com/kodekoding/phastos/v2/go/generator"
)

// =============================================================================
// 1. CSV Generator
// =============================================================================

func Example_csvGenerator() {
	csvGen := generator.NewCSV()
	csvGen.
		SetFileName("output/report"). // akan jadi "output/report.csv"
		SetHeader([]string{"Name", "Email", "Role"}).
		AppendDataRow([]string{"Alice", "alice@mail.com", "Admin"}).
		AppendDataRow([]string{"Bob", "bob@mail.com", "User"}).
		AppendDataRow([]string{"Charlie", "charlie@mail.com", "User"})

	if err := csvGen.Error(); err != nil {
		log.Fatalf("CSV setup error: %v", err)
	}

	if err := csvGen.Generate(); err != nil {
		log.Fatalf("CSV generate error: %v", err)
	}

	fmt.Println("CSV generated:", csvGen.FileName())
}

// =============================================================================
// 2. Excel Generator
// =============================================================================

func Example_excelGenerator() {
	excelGen := generator.NewExcel(&generator.ExcelOptions{Source: "new"})
	excelGen.
		SetFileName("output/report"). // akan jadi "output/report.xlsx"
		SetSheetName("Data Karyawan").
		SetHeader([]string{"Name", "Email", "Role"}).
		AppendDataRow([]string{"Alice", "alice@mail.com", "Admin"}).
		AppendDataRow([]string{"Bob", "bob@mail.com", "User"})

	if err := excelGen.Error(); err != nil {
		log.Fatalf("Excel setup error: %v", err)
	}

	if err := excelGen.Generate(); err != nil {
		log.Fatalf("Excel generate error: %v", err)
	}

	fmt.Println("Excel generated:", excelGen.FileName())
}

// =============================================================================
// 3. PDF Generator
// =============================================================================

func Example_pdfGenerator() {
	pdfGen, err := generator.NewPDF() // default A4, margin 10/11
	if err != nil {
		log.Fatalf("PDF init error: %v", err)
	}

	// custom template function (opsional)
	pdfGen.AddCustomFunction("formatCurrency", func(amount float64) string {
		return fmt.Sprintf("Rp %.0f", amount)
	})

	// data untuk template HTML
	data := map[string]any{
		"InvoiceNumber": "INV-2026-001",
		"CustomerName":  "PT Maju Jaya",
		"Items": []map[string]any{
			{"Name": "Product A", "Qty": 2, "Price": 150000},
			{"Name": "Product B", "Qty": 1, "Price": 300000},
		},
		"Total": 600000,
	}

	fileName := "invoice-001.pdf"
	pdfGen.
		SetTemplate("templates/invoice.html", data).
		SetFooterHTMLTemplate("templates/footer.html"). // opsional
		SetFileName(&fileName)

	if err = pdfGen.Generate(); err != nil {
		log.Fatalf("PDF generate error: %v", err)
	}

	fmt.Println("PDF generated:", pdfGen.FileName())
	// fileName juga sudah ter-update dengan full path (side effect dari SetFileName)
	fmt.Println("Full path:", fileName)
}

// Dengan custom options
func Example_pdfGeneratorCustomOptions() {
	pdfGen, err := generator.NewPDF(&generator.ConverterOptions{
		PageSize:     "Legal",
		MarginTop:    20,
		MarginBottom: 20,
		MarginLeft:   15,
		MarginRight:  15,
	})
	if err != nil {
		log.Fatalf("PDF init error: %v", err)
	}
	_ = pdfGen
}

// =============================================================================
// 4. QR Code Generator
// =============================================================================

func Example_qrGenerator() {
	qrGen, err := generator.NewQR("https://example.com/verify?token=abc123")
	if err != nil {
		log.Fatalf("QR init error: %v", err)
	}

	fileName := "verification-qr"
	qrGen.
		SetFileName(&fileName) // akan jadi "/tmp/qr/<md5>.jpeg"

	if err = qrGen.Generate(); err != nil {
		log.Fatalf("QR generate error: %v", err)
	}

	fmt.Println("QR generated:", qrGen.FileName())
}

// =============================================================================
// 5. Banner Generator
// =============================================================================

func Example_bannerGenerator() {
	banner := generator.NewBanner(
		generator.WithWidth(800),
		generator.WithHeight(400),
		generator.WithBackgroudColor("#1a1a2e"),
	)

	banner.
		AddLabel(&generator.Label{
			Text:     "Welcome to Phastos",
			FontPath: "assets/fonts/Roboto-Bold.ttf",
			Size:     48,
			Color:    color.White,
			XPos:     50,
			YPos:     150,
			Spacing:  1.5,
		}).
		Generate()

	if err := banner.Save("output/banner.png"); err != nil {
		log.Fatalf("Banner save error: %v", err)
	}

	fmt.Println("Banner saved:", banner.FileName())
}

// =============================================================================
// 6. Unified Interface (FileGenerator) — Switch antar tipe generator
// =============================================================================

func Example_unifiedInterface() {
	outputType := "csv" // bisa diganti "excel", "pdf", dll sesuai kebutuhan

	var gen generator.FileGenerator

	switch outputType {
	case "csv":
		csvGen := generator.NewCSV()
		csvGen.
			SetFileName("output/export").
			SetHeader([]string{"ID", "Name"}).
			AppendDataRow([]string{"1", "Alice"}).
			AppendDataRow([]string{"2", "Bob"})
		gen = csvGen

	case "excel":
		excelGen := generator.NewExcel(&generator.ExcelOptions{Source: "new"})
		excelGen.
			SetFileName("output/export").
			SetHeader([]string{"ID", "Name"}).
			AppendDataRow([]string{"1", "Alice"}).
			AppendDataRow([]string{"2", "Bob"})
		gen = excelGen

	case "pdf":
		pdfGen, err := generator.NewPDF()
		if err != nil {
			log.Fatal(err)
		}
		fileName := "export.pdf"
		pdfGen.
			SetTemplate("templates/report.html", map[string]any{"Title": "Report"}).
			SetFileName(&fileName)
		gen = pdfGen
	}

	// dari sini, kode tidak perlu tahu tipe generator apa yang dipakai
	if err := gen.Generate(); err != nil {
		log.Fatalf("Generate error: %v", err)
	}

	fmt.Printf("Generated [%s]: %s\n", outputType, gen.FileName())
}

// =============================================================================
// 7. Bulk Generate + Zip
// =============================================================================

func Example_bulkGenerateZip() {
	// siapkan beberapa generator
	csv1 := generator.NewCSV()
	csv1.SetFileName("output/report-jan").
		SetHeader([]string{"Date", "Amount"}).
		AppendDataRow([]string{"2026-01-01", "100000"}).
		AppendDataRow([]string{"2026-01-15", "250000"})

	csv2 := generator.NewCSV()
	csv2.SetFileName("output/report-feb").
		SetHeader([]string{"Date", "Amount"}).
		AppendDataRow([]string{"2026-02-01", "175000"})

	excelGen := generator.NewExcel(&generator.ExcelOptions{Source: "new"})
	excelGen.SetFileName("output/summary").
		SetHeader([]string{"Month", "Total"}).
		AppendDataRow([]string{"January", "350000"}).
		AppendDataRow([]string{"February", "175000"})

	// bulk generate dengan 3 concurrent workers, lalu zip
	bulk := generator.NewBulkGenerator(generator.WithBulkWorker(3))
	bulk.Add(csv1, csv2, excelGen)

	zipBytes, results, err := bulk.GenerateZip("reports.zip")
	if err != nil {
		log.Fatalf("Zip error: %v", err)
	}

	// cek hasil per-file
	for _, r := range results {
		if r.Error != nil {
			fmt.Printf("FAILED: %s -> %v\n", r.FilePath, r.Error)
		} else {
			fmt.Printf("OK: %s\n", r.FilePath)
		}
	}

	// zipBytes bisa langsung dikirim sebagai HTTP response
	fmt.Printf("Zip size: %d bytes\n", len(zipBytes))

	// contoh kirim zip sebagai HTTP response (dalam handler):
	_ = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="reports.zip"`)
		w.Write(zipBytes)
	}
}

// =============================================================================
// 8. Bulk Generate tanpa Zip (GenerateAll saja)
// =============================================================================

func Example_bulkGenerateAll() {
	csv1 := generator.NewCSV()
	csv1.SetFileName("output/data1").
		SetHeader([]string{"Col1", "Col2"}).
		AppendDataRow([]string{"a", "b"})

	csv2 := generator.NewCSV()
	csv2.SetFileName("output/data2").
		SetHeader([]string{"Col1", "Col2"}).
		AppendDataRow([]string{"c", "d"})

	bulk := generator.NewBulkGenerator()
	bulk.Add(csv1, csv2)

	results := bulk.GenerateAll()
	for _, r := range results {
		if r.Error != nil {
			fmt.Fprintf(os.Stderr, "Error generating %s: %v\n", r.FilePath, r.Error)
			continue
		}
		fmt.Println("Generated:", r.FilePath)
	}
}
