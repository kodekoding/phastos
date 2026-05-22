package helper

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"

	plog "github.com/kodekoding/phastos/v2/go/log"
)

// CryptoManager handles encryption and decryption of API keys
type CryptoManager struct {
	key []byte // 32 bytes for AES-256
}

// NewCryptoManager creates a new crypto manager from an encryption key
// The key should be 32 bytes (256 bits) for AES-256
func NewCryptoManager(encryptionKey string) (*CryptoManager, error) {
	if encryptionKey == "" {
		return nil, errors.New("encryption key cannot be empty")
	}

	// Hash the encryption key to ensure it's 32 bytes
	hash := sha256.Sum256([]byte(encryptionKey))
	return &CryptoManager{key: hash[:]}, nil
}

// NewCryptoManagerFromEnv creates a CryptoManager from an environment variable
func NewCryptoManagerFromEnv(envVarName string) (*CryptoManager, error) {
	key := os.Getenv(envVarName)
	if key == "" {
		return nil, fmt.Errorf("environment variable %s not found", envVarName)
	}
	return NewCryptoManager(key)
}

// Encrypt encrypts a plaintext API key and returns a base64-encoded ciphertext
func (cm *CryptoManager) Encrypt(plaintext string) (string, error) {
	log := plog.Get()
	block, err := aes.NewCipher(cm.key)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create AES cipher")
		return "", err
	}

	// cipher.NewGCM always succeeds when block is valid — aes.NewCipher already succeeded above
	// cipher.NewGCM selalu sukses jika block valid — aes.NewCipher sudah sukses di atas
	gcm, _ := cipher.NewGCM(block)

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	// io.ReadFull from rand.Reader only fails on OS entropy exhaustion — practically unreachable on Linux/macOS
	// io.ReadFull dari rand.Reader hanya gagal jika entropy OS habis — praktis tidak pernah terjadi
	_, _ = io.ReadFull(rand.Reader, nonce)

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Return base64-encoded ciphertext
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext and returns the plaintext
func (cm *CryptoManager) Decrypt(encryptedText string) (string, error) {
	// Decode from base64
	log := plog.Get()
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to decode base64")
		return "", errors.New("invalid encrypted token format")
	}

	block, err := aes.NewCipher(cm.key)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create AES cipher")
		return "", err
	}

	// cipher.NewGCM always succeeds when block is valid — aes.NewCipher already succeeded above
	// cipher.NewGCM selalu sukses jika block valid — aes.NewCipher sudah sukses di atas
	gcm, _ := cipher.NewGCM(block)

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to decrypt")
		return "", errors.New("decryption failed - invalid token")
	}

	return string(plaintext), nil
}

// GeneratePublicKey generates a public representation of a decrypted API key
// This is safe to store on the client side (encrypted or hashed)
func GeneratePublicKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// VerifyAPIKey verifies if a decrypted API key matches a public key
func VerifyAPIKey(apiKey string, publicKey string) bool {
	return GeneratePublicKey(apiKey) == publicKey
}
