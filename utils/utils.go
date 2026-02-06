package utils

import (
	"encoding/base64"
	"errors"
	"io"
	"net/url"
	"strings"

	"github.com/pippellia-btc/blossom"
)

// ParseHash extracts the SHA-256 hash and the optional extension from a URL path.
// The path may optionally start with a leading "/", which is stripped before parsing.
// If the path contains a ".", everything after the first dot is treated as the extension
// (e.g. "hash.tar.gz" yields ext "tar.gz").
func ParseHashExt(path string) (hash blossom.Hash, ext string, err error) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, ".", 2) // separate hash from extension

	hash, err = blossom.ParseHash(parts[0])
	if err != nil {
		return blossom.Hash{}, "", err
	}

	if len(parts) > 1 {
		ext = parts[1]
	}
	return hash, ext, nil
}

// ValidateBlossomURL checks whether the provided URL contains a valid blossom hash in its path.
func ValidateBlossomURL(url *url.URL) error {
	path := strings.TrimPrefix(url.Path, "/")
	parts := strings.SplitN(path, ".", 2) // separate hash from extension
	_, err := blossom.ParseHash(parts[0])
	return err
}

// ValidateHostname checks whether the provided hostname is a valid hostname.
func ValidateHostname(hostname string) error {
	if hostname == "" {
		return errors.New("hostname must not be empty")
	}
	if strings.Contains(hostname, "://") {
		return errors.New("hostname must not include a scheme (e.g. use \"cdn.example.com\" instead of \"https://cdn.example.com\")")
	}

	u, err := url.Parse("https://" + hostname)
	if err != nil {
		return errors.New("invalid hostname: " + err.Error())
	}
	if u.Host != hostname {
		return errors.New("hostname must be a valid domain without path, query, or fragment")
	}
	return nil
}

// ReadNoMore reads at most limit bytes from the reader.
// If the reader contains more than limit bytes, it returns a "body too large" error.
func ReadNoMore(r io.Reader, limit int) ([]byte, *blossom.Error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(limit+1)))
	if err != nil {
		return nil, blossom.ErrBadRequest("failed to read body: " + err.Error())
	}
	if len(data) > limit {
		return nil, blossom.ErrTooLarge("body too large")
	}
	return data, nil
}

// DecodeBase64 detects the base64 encoding variant (standard vs URL-safe,
// padded vs raw) and decodes accordingly.
func DecodeBase64(s string) ([]byte, error) {
	isURLSafe := strings.ContainsAny(s, "-_")
	isStandard := strings.ContainsAny(s, "+/")
	isPadded := strings.HasSuffix(s, "=")

	if isURLSafe && isStandard {
		return nil, errors.New("ambiguous base64: contains both standard (+/) and URL-safe (-_) characters")
	}

	switch {
	case isURLSafe && isPadded:
		return base64.URLEncoding.DecodeString(s)
	case isURLSafe && !isPadded:
		return base64.RawURLEncoding.DecodeString(s)
	case !isURLSafe && isPadded:
		return base64.StdEncoding.DecodeString(s)
	default:
		// No distinguishing characters and no padding.
		// RawStdEncoding is the safest default: it accepts the
		// common alphabet and doesn't require trailing '='.
		return base64.RawStdEncoding.DecodeString(s)
	}
}
