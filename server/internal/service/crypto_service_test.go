package service

import (
	"fmt"
	"testing"
)

func TestCryptoService_EncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	copy(key, []byte("test-master-key-32-bytes-long!"))

	svc := NewCryptoService(key)

	input := map[string]any{
		"api_key": "secret123",
		"region":  "us-east-1",
	}

	encrypted, err := svc.EncryptConfig(input, "test-aad")
	if err != nil {
		t.Fatalf("EncryptConfig failed: %v", err)
	}

	if encrypted == nil || len(encrypted) == 0 {
		t.Fatal("encrypted config should not be empty")
	}

	decrypted, err := svc.DecryptConfig(encrypted, "test-aad")
	if err != nil {
		t.Fatalf("DecryptConfig failed: %v", err)
	}

	if decrypted["api_key"] != "secret123" {
		t.Errorf("expected api_key=secret123, got %v", decrypted["api_key"])
	}
	if decrypted["region"] != "us-east-1" {
		t.Errorf("expected region=us-east-1, got %v", decrypted["region"])
	}
}

func TestCryptoService_DifferentAAD(t *testing.T) {
	key := make([]byte, 32)
	copy(key, []byte("test-master-key-32-bytes-long!"))

	svc := NewCryptoService(key)

	input := map[string]any{"token": "abc"}

	enc1, _ := svc.EncryptConfig(input, "user:1:provider:cloudflare")
	enc2, _ := svc.EncryptConfig(input, "user:2:provider:cloudflare")

	// Different AAD should produce different ciphertext
	if fmt.Sprintf("%v", enc1) == fmt.Sprintf("%v", enc2) {
		t.Error("same input with different AAD should produce different ciphertexts")
	}
}

func TestCryptoService_DecryptWrongAAD(t *testing.T) {
	key := make([]byte, 32)
	copy(key, []byte("test-master-key-32-bytes-long!"))

	svc := NewCryptoService(key)

	input := map[string]any{"token": "secret"}
	encrypted, _ := svc.EncryptConfig(input, "correct-aad")

	_, err := svc.DecryptConfig(encrypted, "wrong-aad")
	if err == nil {
		t.Error("decrypt with wrong AAD should fail")
	}
}
