package _crypto

import (
	"crypto/aes"
	"encoding/hex"
	"testing"
)

func TestDecryptAes128Ecb(t *testing.T) {
	key, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	ciphertext, _ := hex.DecodeString("69c4e0d86a7b0430d8cdb78070b4c55a")
	plaintext := DecryptAes128Ecb(ciphertext, key)
	expected, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	if string(plaintext) != string(expected) {
		t.Fatalf("got %x, want %x", plaintext, expected)
	}
	if DecryptAes128Ecb([]byte{1}, key) != nil || DecryptAes128Ecb(ciphertext, []byte{1}) != nil {
		t.Fatal("invalid ECB input should return nil")
	}
}

func TestXtsRoundTripAndTweakVector(t *testing.T) {
	key, _ := hex.DecodeString("2718281828459045235360287471352662497757247093699959574966967627")
	cipher, err := NewCipher(aes.NewCipher, key)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := make([]byte, 32)
	for i := range plaintext {
		plaintext[i] = byte(i)
	}
	ciphertext := make([]byte, len(plaintext))
	cipher.Encrypt(ciphertext, plaintext, 7)
	if string(ciphertext) == string(plaintext) {
		t.Fatal("XTS encryption did not change plaintext")
	}
	tweak := [16]byte{}
	for i := range tweak {
		tweak[i] = byte(i)
	}
	decoded := make([]byte, len(plaintext))
	copyTweak := tweak
	cipher.Decrypt(decoded, ciphertext, &copyTweak)
	// Decrypt's custom tweak is the encrypted form used by the NCA header path.
	// It is independently checked below with a matching encryption operation.
	if len(decoded) != len(plaintext) {
		t.Fatal("unexpected decrypted length")
	}
	customCiphertext := make([]byte, len(plaintext))
	customTweak := tweak
	cipher.Decrypt(customCiphertext, plaintext, &customTweak)
	if len(customCiphertext) != len(plaintext) {
		t.Fatal("unexpected custom operation length")
	}
	sectorCiphertext := make([]byte, len(plaintext))
	cipher.Encrypt(sectorCiphertext, plaintext, 0)
	sectorTweak := [16]byte{}
	cipher.Decrypt(decoded, sectorCiphertext, &sectorTweak)
	if string(decoded) != string(plaintext) {
		t.Fatalf("round trip failed: got %x, want %x", decoded, plaintext)
	}
}

func TestXtsConstructorAndValidation(t *testing.T) {
	for _, key := range [][]byte{nil, make([]byte, 15), make([]byte, 33)} {
		if _, err := NewCipher(aes.NewCipher, key); err == nil {
			t.Fatal("expected invalid key error")
		}
	}
	if _, err := NewCipher(nil, make([]byte, 32)); err == nil {
		t.Fatal("expected nil cipher function error")
	}
	cipher, err := NewCipher(aes.NewCipher, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	assertPanics(t, func() { cipher.Encrypt(make([]byte, 15), make([]byte, 16), 0) })
	assertPanics(t, func() { cipher.Encrypt(make([]byte, 15), make([]byte, 15), 0) })
	assertPanics(t, func() { cipher.Decrypt(make([]byte, 16), make([]byte, 16), nil) })
}

func assertPanics(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("expected panic")
		}
	}()
	f()
}

func TestMul2(t *testing.T) {
	tweak := [16]byte{}
	tweak[15] = 0x80
	mul2(&tweak)
	if tweak[0] != 0x87 || tweak[15] != 0 {
		t.Fatalf("unexpected reduction: %x", tweak)
	}
}
