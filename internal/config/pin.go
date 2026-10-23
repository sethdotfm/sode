package config

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const pinIter = 600_000

// CheckPinRules says whether pin is acceptable as a new settings PIN.
func CheckPinRules(pin string) error {
	if n := utf8.RuneCountInString(pin); n < 4 || n > 64 {
		return fmt.Errorf("settings PIN must be 4-64 characters")
	}
	return nil
}

// HashPin returns "pbkdf2-sha256$<iter>$<salt>$<hash>" (raw base64).
func HashPin(pin string) (string, error) {
	salt := make([]byte, 16)
	rand.Read(salt)
	key, err := pbkdf2.Key(sha256.New, pin, salt, pinIter, 32)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pinIter, enc(salt), enc(key)), nil
}

// CheckPin reports whether pin matches a HashPin hash.
func CheckPin(hash, pin string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 {
		return false
	}
	dec := base64.RawStdEncoding.DecodeString
	salt, err1 := dec(parts[2])
	want, err2 := dec(parts[3])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pin, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}
