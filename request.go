package blossy

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/pippellia-btc/blossom"
)

type Request interface {
	// ID is the unique identified of the request, useful for logging or tracking.
	ID() int64

	// IP address where the request come from.
	// For rate-limiting purposes you should use [IP.Group] or [IP.GroupPrefix]
	// as a normalized representation of the IP.
	IP() IP

	// Pubkey that signed a valid authorization event if present, otherwise "".
	Pubkey() string

	// IsAuthed returns whether the request has a valid authorization event.
	// It's a shorter version of Request.Pubkey() != "".
	IsAuthed() bool

	// Context returns the context of the underlying [http.Request].
	Context() context.Context

	// Raw returns the underlying [http.Request] as it was received.
	Raw() *http.Request
}

type request struct {
	id     int64
	ip     IP
	pubkey string
	raw    *http.Request
}

func (r request) ID() int64                { return r.id }
func (r request) IP() IP                   { return r.ip }
func (r request) Pubkey() string           { return r.pubkey }
func (r request) IsAuthed() bool           { return r.pubkey != "" }
func (r request) Context() context.Context { return r.raw.Context() }
func (r request) Raw() *http.Request       { return r.raw }

type fetchRequest struct {
	request
	hash blossom.Hash
	ext  string
}

func parseFetch(r *http.Request) (fetchRequest, *blossom.Error) {
	hash, ext, err := ParseHash(r.URL.Path)
	if err != nil {
		return fetchRequest{}, &blossom.Error{Code: http.StatusBadRequest, Reason: err.Error()}
	}

	pubkey, err := parsePubkey(r.Header, VerbGet, hash)
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return fetchRequest{}, &blossom.Error{Code: http.StatusUnauthorized, Reason: err.Error()}
	}

	request := fetchRequest{
		request: request{
			ip:     GetIP(r),
			pubkey: pubkey,
			raw:    r,
		},
		hash: hash,
		ext:  ext,
	}
	return request, nil
}

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
