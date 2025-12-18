package blossy

import (
	"context"
	"errors"
	"net/http"
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
	// It's a shorter version for Request.Pubkey() != "".
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

type blobRequest struct {
	request
	hash string
	ext  string
}

func parseBlobRequest(r *http.Request) (blobRequest, *Error) {
	hash, ext, err := ParseHash(r.URL.Path)
	if err != nil {
		return blobRequest{}, &Error{Code: http.StatusBadRequest, Reason: err.Error()}
	}

	pubkey, err := parsePubkey(r.Header, VerbGet, hash)
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return blobRequest{}, &Error{Code: http.StatusUnauthorized, Reason: err.Error()}
	}

	request := blobRequest{
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
