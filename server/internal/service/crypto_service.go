package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"gorm.io/datatypes"
)

type CryptoService struct {
	masterKey []byte
}

func NewCryptoService(masterKey []byte) *CryptoService {
	return &CryptoService{masterKey: masterKey}
}

func (s *CryptoService) EncryptConfig(input map[string]any, aad string) (datatypes.JSONMap, error) {
	plaintext, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, []byte(aad))
	return datatypes.JSONMap{
		"version":    1,
		"alg":        "AES-256-GCM",
		"nonce":      base64.StdEncoding.EncodeToString(nonce),
		"ciphertext": base64.StdEncoding.EncodeToString(ciphertext),
		"aad":        aad,
	}, nil
}

func (s *CryptoService) DecryptConfig(input datatypes.JSONMap, aad string) (map[string]any, error) {
	nonceText, _ := input["nonce"].(string)
	cipherText, _ := input["ciphertext"].(string)
	if nonceText == "" || cipherText == "" {
		return nil, fmt.Errorf("encrypted config is incomplete")
	}

	nonce, err := base64.StdEncoding.DecodeString(nonceText)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(aad))
	if err != nil {
		return nil, fmt.Errorf("decrypt config: %w", err)
	}

	var output map[string]any
	if err := json.Unmarshal(plaintext, &output); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if output == nil {
		output = map[string]any{}
	}
	return output, nil
}
