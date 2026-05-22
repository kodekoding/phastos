package common

import (
	"bytes"
	"sync"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// GetBuffer returns a *bytes.Buffer from the pool, writes the provided data into it, and resets its state.
func GetBuffer(data []byte) *bytes.Buffer {
	b := bufPool.Get().(*bytes.Buffer) //nolint:errcheck
	b.Reset()
	if data != nil {
		b.Write(data)
	}
	return b
}

// PutBuffer returns the buffer to the pool after resetting it.
func PutBuffer(b *bytes.Buffer) {
	b.Reset()
	bufPool.Put(b)
}
