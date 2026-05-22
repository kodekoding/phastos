package importer

import (
	"strings"
	"sync"
)

// builderPool pools strings.Builder objects used for building pivot keys.
var builderPool = sync.Pool{
	New: func() any {
		return new(strings.Builder)
	},
}

// getBuilder retrieves a strings.Builder from the pool and resets it.
func getBuilder() *strings.Builder {
	//nolint:errcheck // Pool.New always returns *strings.Builder
	b := builderPool.Get().(*strings.Builder)
	b.Reset()
	return b
}

// putBuilder returns a strings.Builder to the pool after resetting it.
func putBuilder(b *strings.Builder) {
	b.Reset()
	builderPool.Put(b)
}

// processedResultPool pools processedResult objects to reduce per-row allocations.
var processedResultPool = sync.Pool{
	New: func() any {
		return new(processedResult)
	},
}

// getProcessedResult retrieves a processedResult from the pool with zeroed fields.
func getProcessedResult() *processedResult {
	//nolint:errcheck // Pool.New always returns *processedResult
	pr := processedResultPool.Get().(*processedResult)
	pr.ParsedStruct = nil
	pr.RawData = nil
	pr.Error = nil
	return pr
}

// putProcessedResult returns a processedResult to the pool after zeroing its fields.
func putProcessedResult(pr *processedResult) {
	pr.ParsedStruct = nil
	pr.RawData = nil
	pr.Error = nil
	processedResultPool.Put(pr)
}
