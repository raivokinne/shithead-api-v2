package utils

import (
	"crypto/rand"
	"encoding/base64"
)

func GenerateToken() string {
	bytes := make([]byte, 24)
	_, err := rand.Read(bytes)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
