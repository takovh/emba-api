package sse

import (
	"crypto/rand"
	"encoding/hex"
)

func randomID(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
