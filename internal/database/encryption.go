package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Encryptor handles AES-256-GCM encryption/decryption for database-stored secrets.
// Uses the master encryption key (the only secret not stored in the database).
type Encryptor struct {
	gcm cipher.AEAD
}

// NewEncryptor creates a new AES-256-GCM encryptor from a master key.
// The key must be exactly 32 bytes (256 bits) for AES-256.
func NewEncryptor(masterKey string) (*Encryptor, error) {
	keyBytes := []byte(masterKey)

	// Ensure key is exactly 32 bytes for AES-256
	if len(keyBytes) < 32 {
		return nil, fmt.Errorf("master encryption key must be at least 32 bytes, got %d", len(keyBytes))
	}
	keyBytes = keyBytes[:32]

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	return &Encryptor{gcm: gcm}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64-encoded string.
// Each call generates a unique random nonce, so encrypting the same plaintext
// twice produces different ciphertext (this is correct and expected).
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	// Seal appends the encrypted data to the nonce
	// Format: nonce + ciphertext + GCM tag
	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext.
func (e *Encryptor) Decrypt(encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding base64: %w", err)
	}

	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}

	return string(plaintext), nil
}

// EncryptBytes encrypts raw bytes and returns the encrypted bytes (not base64).
// Useful for storing directly in bytea columns.
func (e *Encryptor) EncryptBytes(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	return e.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptBytes decrypts raw encrypted bytes.
func (e *Encryptor) DecryptBytes(ciphertext []byte) ([]byte, error) {
	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return e.gcm.Open(nil, nonce, ciphertext, nil)
}
