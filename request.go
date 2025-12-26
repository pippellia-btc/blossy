package blossy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
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

type uploadRequest struct {
	request
	hints UploadHints
	body  io.ReadCloser
}

func parseUpload(r *http.Request) (uploadRequest, *blossom.Error) {
	// In the future I want to pass the hash of the body.
	// Now there is no point since the auth scheme is broken anyway.
	// See https://github.com/hzrd149/blossom/pull/87
	pubkey, err := parsePubkey(r.Header, VerbUpload, blossom.Hash{})
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return uploadRequest{}, &blossom.Error{Code: http.StatusUnauthorized, Reason: err.Error()}
	}

	hints := UploadHints{
		Type: r.Header.Get("Content-Type"),
		Size: -1, // default to unknown
	}

	if cl := r.Header.Get("Content-Length"); cl != "" {
		size, err := strconv.ParseInt(cl, 10, 64)
		if err == nil {
			hints.Size = size
		}
	}

	request := uploadRequest{
		request: request{
			ip:     GetIP(r),
			pubkey: pubkey,
			raw:    r,
		},
		hints: hints,
		body:  r.Body,
	}
	return request, nil
}

func parseUploadCheck(r *http.Request) (uploadRequest, *blossom.Error) {
	// In the future I want to pass the hash of the body.
	// Now there is no point since the auth scheme is broken anyway.
	// See https://github.com/hzrd149/blossom/pull/87
	pubkey, err := parsePubkey(r.Header, VerbUpload, blossom.Hash{})
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return uploadRequest{}, &blossom.Error{Code: http.StatusUnauthorized, Reason: err.Error()}
	}

	sha256 := r.Header.Get("X-SHA-256")
	if sha256 == "" {
		return uploadRequest{}, &blossom.Error{Code: http.StatusBadRequest, Reason: "'X-SHA-256' header is missing or empty"}
	}
	hash, err := blossom.ParseHash(sha256)
	if err != nil {
		return uploadRequest{}, &blossom.Error{Code: http.StatusBadRequest, Reason: "'X-SHA-256' header is invalid: " + err.Error()}
	}

	cl := r.Header.Get("X-Content-Length")
	if cl == "" {
		return uploadRequest{}, &blossom.Error{Code: http.StatusBadRequest, Reason: "'X-Content-Length' header is missing or empty"}
	}
	size, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return uploadRequest{}, &blossom.Error{Code: http.StatusBadRequest, Reason: "'X-Content-Length' header is invalid: " + err.Error()}
	}

	mime := r.Header.Get("X-Content-Type")
	if mime == "" {
		return uploadRequest{}, &blossom.Error{Code: http.StatusBadRequest, Reason: "'X-Content-Type' header is missing or empty"}
	}

	request := uploadRequest{
		request: request{
			ip:     GetIP(r),
			pubkey: pubkey,
			raw:    r,
		},
		hints: UploadHints{
			Hash: hash,
			Type: mime,
			Size: size,
		},
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
