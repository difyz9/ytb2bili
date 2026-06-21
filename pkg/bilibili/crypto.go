package bilibili

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/zolagz/ytb2bili/pkg/store/model"
)

// Encrypt encrypts sensitive data for persistence.
func (s *Service) Encrypt(plaintext string) (string, error) {
	return s.encrypt(plaintext)
}

func (s *Service) encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *Service) decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, encrypted := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GetDecryptedCookies returns decrypted cookies JSON.
func (s *Service) GetDecryptedCookies(account *model.AccountBinding) (string, error) {
	return s.decrypt(account.Cookies)
}

// GetDecryptedTokens returns decrypted access and refresh tokens.
func (s *Service) GetDecryptedTokens(account *model.AccountBinding) (accessToken, refreshToken string, err error) {
	accessToken, err = s.decrypt(account.AccessToken)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = s.decrypt(account.RefreshToken)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}