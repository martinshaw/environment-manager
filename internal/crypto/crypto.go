package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
)

// KeyFile is the passphrase-wrapped data key stored in the envm directory.
const KeyFile = "key.age"

// RecoveryFile is the same data key wrapped for the one-time recovery identity.
// The identity itself is not stored.
const RecoveryFile = "recovery.age"

// MinPassphraseLen is the shortest accepted vault passphrase.
const MinPassphraseLen = 10

// Hash is the hex SHA-256 of b. A nil slice hashes the same as an empty slice.
func Hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Create generates a data key, wraps it for passphrase and a one-time recovery
// identity, and writes key.age. work is the scrypt logN used for the
// passphrase recipient; zero selects the age default. The recovery string is
// returned once and is not stored.
func Create(dir, passphrase string, work int) (key []byte, recovery string, err error) {
	if len(passphrase) < MinPassphraseLen {
		return nil, "", fmt.Errorf("passphrase must be at least %d characters", MinPassphraseLen)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", err
	}
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return nil, "", err
	}
	if work > 0 {
		recipient.SetWorkFactor(work)
	}
	ident, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, "", err
	}
	wrapped, err := encryptTo(raw, recipient)
	if err != nil {
		return nil, "", err
	}
	recoveryWrapped, err := encryptTo(raw, ident.Recipient())
	if err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(filepath.Join(dir, KeyFile), wrapped, 0o600); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(filepath.Join(dir, RecoveryFile), recoveryWrapped, 0o600); err != nil {
		return nil, "", err
	}
	return raw, ident.String(), nil
}

func encryptTo(plain []byte, recipient age.Recipient) ([]byte, error) {
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plain); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnlockPassphrase unwraps the data key with a passphrase.
func UnlockPassphrase(dir, passphrase string) ([]byte, error) {
	id, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, err
	}
	return decryptKey(dir, id)
}

// UnlockRecovery unwraps the data key with the one-time recovery identity.
func UnlockRecovery(dir, recovery string) ([]byte, error) {
	id, err := age.ParseX25519Identity(strings.TrimSpace(recovery))
	if err != nil {
		return nil, errors.New("invalid recovery key")
	}
	return decryptFile(dir, RecoveryFile, id)
}

func decryptKey(dir string, id age.Identity) ([]byte, error) {
	return decryptFile(dir, KeyFile, id)
}

func decryptFile(dir, name string, id age.Identity) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	r, err := age.Decrypt(bytes.NewReader(data), id)
	if err != nil {
		return nil, errors.New("could not unlock vault")
	}
	key, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("invalid data key")
	}
	return key, nil
}

// Seal encrypts plain with AES-GCM. The result is nonce followed by ciphertext.
func Seal(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return append(nonce, gcm.Seal(nil, nonce, plain, nil)...), nil
}

// Open decrypts a blob produced by Seal.
func Open(key, blob []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(blob) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := blob[:gcm.NonceSize()], blob[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("could not decrypt version")
	}
	return plain, nil
}
