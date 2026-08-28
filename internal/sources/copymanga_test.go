package sources

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"testing"
)

// encryptCopyMangaStyle mirrors CopyManga's scheme: ASCII IV of 16 chars
// prepended to hex-encoded AES-128-CBC ciphertext with PKCS7 padding.
func encryptCopyMangaStyle(t *testing.T, plaintext []byte, keyStr string) string {
	t.Helper()

	key := []byte(keyStr)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes new cipher: %v", err)
	}

	pad := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := append(append([]byte{}, plaintext...), bytes.Repeat([]byte{byte(pad)}, pad)...)

	iv := []byte("0123456789abcdef")
	if len(iv) != aes.BlockSize {
		t.Fatalf("bad iv length")
	}

	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, padded)

	return string(iv) + hex.EncodeToString(encrypted)
}

func TestDecryptAESRoundTrip(t *testing.T) {
	key := "op0zzpvv.nmn.00p"
	payloads := []string{
		`{"url":"https://img.example.com/1.jpg"}`,
		`[{"url":"a"},{"url":"b"}]`,
		"",
	}

	for _, p := range payloads {
		enc := encryptCopyMangaStyle(t, []byte(p), key)
		got, err := DecryptAES(enc, key)
		if err != nil {
			t.Fatalf("DecryptAES(%q) error: %v", p, err)
		}
		if string(got) != p {
			t.Fatalf("DecryptAES round trip mismatch: got %q want %q", got, p)
		}
	}
}

func TestDecryptAESRejectsShortInput(t *testing.T) {
	if _, err := DecryptAES("abc", "op0zzpvv.nmn.00p"); err == nil {
		t.Fatal("expected error for too-short payload")
	}
}
