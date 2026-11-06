package main

import (
	"crypto/sha256"
	"encoding/hex"
)

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
