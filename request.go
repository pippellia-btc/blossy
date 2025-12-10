package blossom

import (
	"context"
	"io"
	"net/http"
)

type Request interface {
	// ID is the unique identified of the request, useful for logging or tracking.
	ID() int64

	// Context returns the context of the underlying [http.Request].
	Context() context.Context

	// IP address where the request come from.
	// For rate-limiting purposes you should use [IP.Group] or [IP.GroupPrefix]
	// as a normalized representation of the IP.
	IP() IP

	// Pubkey that signed a valid authorization event if present, otherwise "".
	Pubkey() string

	// IsAuthed returns whether the request has a valid authorization event.
	// It's a shorter version for Request.Pubkey() != "".
	IsAuthed() bool

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
func (r request) Context() context.Context { return r.raw.Context() }
func (r request) IP() IP                   { return r.ip }
func (r request) Pubkey() string           { return r.pubkey }
func (r request) IsAuthed() bool           { return r.pubkey != "" }
func (r request) Raw() *http.Request       { return r.raw }

// Blob is the fundamental data structure of blossom.
type Blob struct {
	Data io.ReadCloser
	MIME string
	Size int64
}
