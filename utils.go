package blossy

import (
	"net/http"
	"strings"

	"github.com/pippellia-btc/blossom"
)

// ParseHash extracts the SHA256 hash from URL path.
// Supports both /<sha256> and /<sha256>.<ext> formats.
func ParseHash(path string) (hash blossom.Hash, ext string, err error) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, ".", 2) // separate hash from extention

	hash, err = blossom.ParseHash(parts[0])
	if err != nil {
		return blossom.Hash{}, "", err
	}

	if len(parts) > 1 {
		ext = parts[1]
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
