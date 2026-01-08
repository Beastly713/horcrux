package encryptor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// Encrypt performs AES-GCM encryption on the plaintext using the provided key.
// It returns a byte slice containing the Nonce appended with the Ciphertext (and Tag).
func Encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Never use a static IV/Nonce (unlike the original approach which used a zero-value IV).
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal appends the ciphertext and the authentication tag to the nonce.
	// Format: [Nonce | Ciphertext | Tag]
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt performs AES-GCM decryption.
// It expects the input to be in the format [Nonce | Ciphertext | Tag].
// It returns an error if authentication fails (integrity check).
func Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	// Split the nonce from the actual ciphertext
	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption/authentication failed: %w", err)
	}

	return plaintext, nil
}