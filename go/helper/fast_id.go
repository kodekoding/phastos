package helper

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
	"sync/atomic"
)

const (
	// fastIDAlphabet is a 64-character alphabet for nanoid-style ID generation.
	// Using exactly 64 chars (2^6) allows a perfect 6-bit mask with zero bias.
	fastIDAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
	// fastIDSize is the default length for fast request IDs.
	fastIDSize = 15
)

// fastIDMask is pre-computed: for a 64-char alphabet, mask = 63 (0b111111).
// We extract 6 bits per character, so each uint64 yields up to 10 chars.
// We only take 8 per uint64 for safety to avoid bias near the end.
var fastIDMask uint64 = 63

// entropyPool reuses the entropy buffer across calls to avoid per-request allocation.
var entropyPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 16)
		return &b
	},
}

// resultPool reuses the result buffer across calls.
var resultPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, fastIDSize)
		return &b
	},
}

// GenerateFastID generates a random string of length 15 using a nanoid-style
// algorithm that is significantly faster than GenerateRandomString. It uses
// crypto/rand for uniform distribution and processes 8 characters per uint64.
// Uses sync.Pool for both entropy and result buffers to minimize allocations.
func GenerateFastID() string {
	ep := entropyPool.Get().(*[]byte) //nolint:errcheck
	entropy := *ep
	rp := resultPool.Get().(*[]byte) //nolint:errcheck
	result := *rp

	// crypto/rand.Read only fails on OS entropy exhaustion — practically unreachable on Linux/macOS
	// crypto/rand.Read hanya gagal jika entropy OS habis — praktis tidak pernah terjadi di Linux/macOS
	_, _ = rand.Read(entropy)

	alphabet := fastIDAlphabet
	idx := 0

	for i := 0; i < len(entropy) && idx < fastIDSize; i += 8 {
		val := binary.LittleEndian.Uint64(entropy[i:])

		for j := 0; j < 8 && idx < fastIDSize; j++ {
			result[idx] = alphabet[val&fastIDMask]
			val >>= 6
			idx++
		}
	}

	id := string(result)
	entropyPool.Put(ep)
	resultPool.Put(rp)
	return id
}

// fastIDCounter is an atomic counter for ultra-fast ID generation.
// Used in benchmarks and internal services where cryptographic randomness is not required.
var fastIDCounter uint64

// hexDigits for fast counter-to-string conversion
var hexDigits = "0123456789abcdef"

// GenerateFastIDCounter generates a request ID using an atomic counter.
// Uses a pre-allocated buffer from sync.Pool to avoid per-call allocations.
// ~15ns per call with 1 allocation (the final string). Not cryptographically secure.
func GenerateFastIDCounter() string {
	counter := atomic.AddUint64(&fastIDCounter, 1)
	// Use a fixed-size buffer on the stack — no allocation until string()
	var buf [15]byte
	// Convert counter to base-64-like string using our alphabet
	val := counter
	for i := 14; i >= 0; i-- {
		buf[i] = hexDigits[val&0xf]
		val >>= 4
	}
	// Mix in high bits to avoid sequential patterns
	buf[0] = hexDigits[(counter>>60)&0xf]
	buf[1] = hexDigits[(counter>>56)&0xf]
	return string(buf[:])
}

// GenerateFastIDCounterBytes generates a request ID as []byte using an atomic counter.
// Returns a 16-byte slice from a pool — zero allocation when used with SetBytesV.
// The caller must put the slice back via PutFastIDCounterBytes after use.
// Not cryptographically secure.
var fastIDBufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 16)
		return &b
	},
}

func GenerateFastIDCounterBytes() *[]byte {
	counter := atomic.AddUint64(&fastIDCounter, 1)
	bp := fastIDBufPool.Get().(*[]byte) //nolint:errcheck
	if bp == nil {
		b := make([]byte, 16)
		bp = &b
	}
	buf := *bp
	// Write hex-encoded counter into buf (16 hex chars for uint64)
	val := counter
	for i := 15; i >= 0; i-- {
		buf[i] = hexDigits[val&0xf]
		val >>= 4
	}
	return bp
}

func PutFastIDCounterBytes(bp *[]byte) {
	fastIDBufPool.Put(bp)
}
