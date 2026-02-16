package generator

// FileGenerator is the unified contract for all generator types (PDF, CSV, Excel, QR, Banner).
// It provides the common operations shared across all generators, allowing callers to
// switch between generator types without changing the generation/consumption logic.
//
// Usage example:
//
//	var gen generator.FileGenerator
//	switch outputType {
//	case "pdf":
//	    gen = pdfInstance
//	case "csv":
//	    gen = csvInstance
//	case "excel":
//	    gen = excelInstance
//	}
//	if err := gen.Generate(); err != nil { ... }
//	fmt.Println("output:", gen.FileName())
type FileGenerator interface {
	// Generate produces the output file. Returns an error if generation fails.
	Generate() error
	// FileName returns the path/name of the generated output file.
	FileName() string
}
