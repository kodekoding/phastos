package helper

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCryptoManager(t *testing.T) {
	t.Run("should create crypto manager with valid key", func(t *testing.T) {
		cm, err := NewCryptoManager("my-secret-key-for-testing")
		assert.NoError(t, err)
		assert.NotNil(t, cm)
	})

	t.Run("should return error for empty key", func(t *testing.T) {
		cm, err := NewCryptoManager("")
		assert.Error(t, err)
		assert.Nil(t, cm)
		assert.Contains(t, err.Error(), "encryption key cannot be empty")
	})
}

func TestNewCryptoManagerFromEnv(t *testing.T) {
	t.Run("should create crypto manager from env var", func(t *testing.T) {
		os.Setenv("TEST_CRYPTO_KEY", "my-secret-key")
		defer os.Unsetenv("TEST_CRYPTO_KEY")

		cm, err := NewCryptoManagerFromEnv("TEST_CRYPTO_KEY")
		assert.NoError(t, err)
		assert.NotNil(t, cm)
	})

	t.Run("should return error when env var not set", func(t *testing.T) {
		cm, err := NewCryptoManagerFromEnv("NON_EXISTENT_KEY")
		assert.Error(t, err)
		assert.Nil(t, cm)
		assert.Contains(t, err.Error(), "environment variable NON_EXISTENT_KEY not found")
	})
}

func TestEncryptDecrypt(t *testing.T) {
	cm, err := NewCryptoManager("test-encryption-key-32bytes!!")
	assert.NoError(t, err)

	t.Run("should encrypt and decrypt successfully", func(t *testing.T) {
		plaintext := "my-secret-api-key-12345"
		encrypted, err := cm.Encrypt(plaintext)
		assert.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.NotEqual(t, plaintext, encrypted)

		decrypted, err := cm.Decrypt(encrypted)
		assert.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("should produce different ciphertexts for same plaintext", func(t *testing.T) {
		plaintext := "same-input"
		encrypted1, _ := cm.Encrypt(plaintext)
		encrypted2, _ := cm.Encrypt(plaintext)
		// Due to random nonce, same plaintext should produce different ciphertexts
		assert.NotEqual(t, encrypted1, encrypted2)
	})

	t.Run("should handle empty plaintext", func(t *testing.T) {
		encrypted, err := cm.Encrypt("")
		assert.NoError(t, err)
		assert.NotEmpty(t, encrypted)

		decrypted, err := cm.Decrypt(encrypted)
		assert.NoError(t, err)
		assert.Equal(t, "", decrypted)
	})

	t.Run("should handle long plaintext", func(t *testing.T) {
		longText := "this-is-a-very-long-api-key-that-exceeds-normal-length-for-testing-purposes-1234567890"
		encrypted, err := cm.Encrypt(longText)
		assert.NoError(t, err)

		decrypted, err := cm.Decrypt(encrypted)
		assert.NoError(t, err)
		assert.Equal(t, longText, decrypted)
	})

	t.Run("should fail to decrypt invalid base64", func(t *testing.T) {
		_, err := cm.Decrypt("not-valid-base64!!!")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid encrypted token format")
	})

	t.Run("should fail to decrypt tampered ciphertext", func(t *testing.T) {
		encrypted, _ := cm.Encrypt("test-data")
		// Tamper with the encrypted data
		tampered := encrypted[:len(encrypted)-2] + "XX"
		_, err := cm.Decrypt(tampered)
		assert.Error(t, err)
	})

	t.Run("should fail to decrypt with different key", func(t *testing.T) {
		encrypted, _ := cm.Encrypt("secret-data")

		otherCm, _ := NewCryptoManager("different-key-entirely")
		_, err := otherCm.Decrypt(encrypted)
		assert.Error(t, err)
	})
}

func TestEncrypt_AesErrorWithShortKey(t *testing.T) {
	t.Run("should fail Encrypt with short key", func(t *testing.T) {
		cm := &CryptoManager{key: []byte("short")}
		_, err := cm.Encrypt("test")
		assert.Error(t, err)
	})

	t.Run("should fail Decrypt with short key", func(t *testing.T) {
		cm := &CryptoManager{key: []byte("short")}
		_, err := cm.Decrypt("dGVzdA==")
		assert.Error(t, err)
	})
}

func TestGeneratePublicKey(t *testing.T) {
	t.Run("should generate consistent public key", func(t *testing.T) {
		apiKey := "my-api-key"
		pk1 := GeneratePublicKey(apiKey)
		pk2 := GeneratePublicKey(apiKey)
		assert.Equal(t, pk1, pk2)
		assert.NotEmpty(t, pk1)
	})

	t.Run("should generate different public keys for different inputs", func(t *testing.T) {
		pk1 := GeneratePublicKey("key-1")
		pk2 := GeneratePublicKey("key-2")
		assert.NotEqual(t, pk1, pk2)
	})
}

func TestVerifyAPIKey(t *testing.T) {
	t.Run("should verify matching API key", func(t *testing.T) {
		apiKey := "my-api-key"
		publicKey := GeneratePublicKey(apiKey)
		assert.True(t, VerifyAPIKey(apiKey, publicKey))
	})

	t.Run("should reject non-matching API key", func(t *testing.T) {
		publicKey := GeneratePublicKey("correct-key")
		assert.False(t, VerifyAPIKey("wrong-key", publicKey))
	})

	t.Run("should reject empty API key against valid public key", func(t *testing.T) {
		publicKey := GeneratePublicKey("some-key")
		assert.False(t, VerifyAPIKey("", publicKey))
	})
}
