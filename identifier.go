package work

import (
	"crypto/rand"
	"encoding/hex"
)

func makeIdentifier() string {
	b := make([]byte, 12)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
