package helper

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	satoriuuid "github.com/satori/go.uuid"
)

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var (
	seededRand     = rand.New(rand.NewSource(time.Now().UnixNano()))
	intRand        = (*rand.Rand).Intn
	generateUUID   = uuid.Must
	generateUUIDV4 = satoriuuid.NewV4
)

func GenerateRandomStringWithCharset(length int, charset string) string {

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[intRand(seededRand, len(charset))]
	}
	return string(b)
}

func GenerateRandomString(length int) string {
	return GenerateRandomStringWithCharset(length, charset)
}

func GenerateUUID() string {
	return fmt.Sprintf("%s", generateUUID(uuid.New(), nil))
}

func GenerateUUIDV4() string {
	return generateUUIDV4().String()
}
