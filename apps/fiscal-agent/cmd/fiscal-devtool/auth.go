package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/argon2"
)

// InitialDevtoolPIN is the ONLY bootstrap PIN for fiscal-devtool (first run).
const InitialDevtoolPIN = "111111"

var (
	errBadPIN      = errors.New("PIN 必须是 6 位数字")
	errPINMismatch = errors.New("PIN 不正确")
	pinSix         = regexp.MustCompile(`^\d{6}$`)
)

type pinParams struct {
	memory, iterations, saltLength, keyLength uint32
	parallelism                               uint8
}

var pinP = pinParams{memory: 64 * 1024, iterations: 2, parallelism: 1, saltLength: 16, keyLength: 32}

// AuthState is returned by LoginPIN.
type AuthState struct {
	MustChangePIN bool
}

// LoginPIN is the ONLY PIN verify path (reads ToolState from state file).
func LoginPIN(statePath, pin string) (*AuthState, error) {
	st, err := LoadToolState(statePath)
	if err != nil {
		return nil, err
	}
	if !verifyDevtoolPIN(pin, st.PinHash) {
		return nil, errPINMismatch
	}
	return &AuthState{MustChangePIN: st.MustChangePIN}, nil
}

// ChangePIN is the ONLY PIN write path (updates state file; clears must_change_pin).
func ChangePIN(statePath, oldPIN, newPIN string) error {
	if _, err := LoginPIN(statePath, oldPIN); err != nil {
		return err
	}
	h, err := hashDevtoolPIN(newPIN)
	if err != nil {
		return err
	}
	st, err := LoadToolState(statePath)
	if err != nil {
		return err
	}
	st.PinHash = h
	st.MustChangePIN = false
	return SaveToolState(statePath, st)
}

func hashDevtoolPIN(pin string) (string, error) {
	if !pinSix.MatchString(pin) {
		return "", errBadPIN
	}
	salt := make([]byte, pinP.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(pin), salt, pinP.iterations, pinP.memory, pinP.parallelism, pinP.keyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, pinP.memory, pinP.iterations, pinP.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyDevtoolPIN(pin, encoded string) bool {
	if !pinSix.MatchString(pin) {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(pin), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
