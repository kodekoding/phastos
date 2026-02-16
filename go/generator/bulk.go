package generator

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type (
	// BulkResult holds the outcome of a single generator within a bulk operation.
	BulkResult struct {
		FilePath string `json:"file_path"`
		Error    error  `json:"error,omitempty"`
	}

	// BulkOption configures the BulkGenerator.
	BulkOption func(*BulkGenerator)

	// BulkGenerator runs multiple FileGenerator instances concurrently
	// and can compress all successful outputs into a single zip archive.
	BulkGenerator struct {
		generators []FileGenerator
		worker     int
	}
)

// NewBulkGenerator creates a new BulkGenerator. Default worker pool size is 5.
func NewBulkGenerator(opts ...BulkOption) *BulkGenerator {
	bg := &BulkGenerator{
		worker: 5,
	}
	for _, opt := range opts {
		opt(bg)
	}
	return bg
}

// WithBulkWorker sets the number of concurrent workers for bulk generation.
func WithBulkWorker(n int) BulkOption {
	return func(bg *BulkGenerator) {
		if n > 0 {
			bg.worker = n
		}
	}
}

// Add appends one or more FileGenerator to the bulk queue.
func (bg *BulkGenerator) Add(generators ...FileGenerator) *BulkGenerator {
	bg.generators = append(bg.generators, generators...)
	return bg
}

// GenerateAll runs all generators concurrently using a worker pool and returns
// the result for each generator (preserving the original order).
func (bg *BulkGenerator) GenerateAll() []BulkResult {
	results := make([]BulkResult, len(bg.generators))

	jobs := make(chan int, len(bg.generators))
	wg := new(sync.WaitGroup)

	// spawn workers
	for w := 0; w < bg.worker; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				gen := bg.generators[idx]
				err := gen.Generate()
				results[idx] = BulkResult{
					FilePath: gen.FileName(),
					Error:    err,
				}
			}
		}()
	}

	// send jobs
	for i := range bg.generators {
		jobs <- i
	}
	close(jobs)

	wg.Wait()
	return results
}

// GenerateZip runs all generators concurrently, then compresses every successfully
// generated file into a single zip archive. Returns the zip content as bytes.
//
// Files that fail to generate are skipped from the archive but included in the
// returned BulkResult slice so the caller can inspect errors.
func (bg *BulkGenerator) GenerateZip(zipFileName string) ([]byte, []BulkResult, error) {
	results := bg.GenerateAll()

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for _, result := range results {
		if result.Error != nil {
			continue
		}

		filePath := result.FilePath
		if filePath == "" {
			continue
		}

		srcFile, err := os.Open(filePath)
		if err != nil {
			result.Error = err
			continue
		}

		baseName := filepath.Base(filePath)
		w, err := zipWriter.Create(baseName)
		if err != nil {
			srcFile.Close()
			result.Error = err
			continue
		}

		if _, err = io.Copy(w, srcFile); err != nil {
			srcFile.Close()
			result.Error = err
			continue
		}
		srcFile.Close()
	}

	if err := zipWriter.Close(); err != nil {
		return nil, results, err
	}

	return buf.Bytes(), results, nil
}
