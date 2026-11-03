package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strings"
	"unicode/utf8"
)

var passwordSlots = make(chan struct{}, 2)

func ValidPassword(p string) bool {
	return utf8.RuneCountInString(p) >= 12 && utf8.RuneCountInString(p) <= 128 && len(p) <= 512
}
func HashPassword(p string) (string, error) {
	if !ValidPassword(p) {
		return "", fmt.Errorf("Passwort benötigt 12 bis 128 Zeichen")
	}
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	passwordSlots <- struct{}{}
	defer func() { <-passwordSlots }()
	key := argon2.IDKey([]byte(p), salt, 3, 64*1024, 2, 32)
	b := base64.RawStdEncoding
	return "$argon2id$v=19$m=65536,t=3,p=2$" + b.EncodeToString(salt) + "$" + b.EncodeToString(key), nil
}
func passwordHashParts(hash string) (salt, want []byte, valid bool) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=2" {
		return nil, nil, false
	}
	b := base64.RawStdEncoding
	salt, e := b.DecodeString(parts[4])
	if e != nil || len(salt) != 16 {
		return nil, nil, false
	}
	want, e = b.DecodeString(parts[5])
	if e != nil || len(want) != 32 {
		return nil, nil, false
	}
	return salt, want, true
}
func VerifyPassword(hash, p string) bool {
	salt, want, valid := passwordHashParts(hash)
	if !valid || len(p) > 512 {
		return false
	}
	passwordSlots <- struct{}{}
	defer func() { <-passwordSlots }()
	got := argon2.IDKey([]byte(p), salt, 3, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(want, got) == 1
}

// Dummy verification is never an account credential. Eligibility must be checked
// separately even when the supplied password happens to match this work hash.
const dummyLoginHash = "$argon2id$v=19$m=65536,t=3,p=2$MDEyMzQ1Njc4OWFiY2RlZg$930JUgpCj6MEj04UbiN5uo2U3ee9EBFW3O17UL9usdQ"

func checkLoginPassword(storedHash, password string, accountFound bool) (matched, eligible bool) {
	_, _, usable := passwordHashParts(storedHash)
	eligible = accountFound && usable
	hash := storedHash
	if !eligible {
		hash = dummyLoginHash
	}
	matched = VerifyPassword(hash, password)
	return matched, eligible
}
