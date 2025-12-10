package blossom

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const (
	HashLength = 64
)

var (
	HashRegexp = regexp.MustCompile(`[a-fA-F0-9]{64}`)
)

// ParseHash extracts the SHA256 hash from URL path.
// Supports both /<sha256> and /<sha256>.<ext> formats.
func ParseHash(path string) (hash string, ext string, err error) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, ".", 2) // separate hash from extention

	hash = parts[0]
	if len(parts) > 1 {
		ext = "." + parts[1]
	}

	if len(hash) != HashLength {
		return "", "", fmt.Errorf("invalid hash lenght: %d", len(hash))
	}

	if !HashRegexp.MatchString(hash) {
		return "", "", fmt.Errorf("invalid hash: %s", hash)
	}
	return hash, ext, nil
}

// SetCORS sets CORS headers as required by BUD-01.
func SetCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, *")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
}
