package control

import (
	"crypto/rand"
	"encoding/hex"
)

func NewID(prefix string) string {
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	return prefix + "_" + hex.EncodeToString(raw[:])
}
