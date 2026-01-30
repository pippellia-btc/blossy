package blossy

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/pippellia-btc/blossom"
)

// Hooks of the blossom server, that the user of this framework can configure.
type Hooks struct {
	Reject RejectHooks
	On     OnHooks
}

func DefaultHooks() Hooks {
	return Hooks{
		Reject: RejectHooks{},
		On:     NewOnHooks(),
	}
}

// RejectHooks defines optional functions that can preemptively reject
// certain actions before they are processed by the server.
//
// Each function in a hook slice is evaluated in order. If any function
// returns a non-nil error, the corresponding input is immediately rejected.
//
// These hooks are useful for enforcing access policies, validating input,
// or applying rate limits before the server performs further processing.
type RejectHooks struct {
	// FetchBlob is invoked before processing a GET /<hash>.<ext> request.
	FetchBlob slice[func(r Request, hash blossom.Hash, ext string) *blossom.Error]

	// FetchMeta is invoked before processing a HEAD /<hash>.<ext> request.
	FetchMeta slice[func(r Request, hash blossom.Hash, ext string) *blossom.Error]

	// Delete is invoked before processing a DELETE /<hash> request.
	Delete slice[func(r Request, hash blossom.Hash) *blossom.Error]

	// Upload is invoked when processing the HEAD /upload and before processing every PUT /upload request.
	Upload slice[func(r Request, hints UploadHints) *blossom.Error]

	// Mirror is invoked before processing a PUT /mirror request.
	// The url has been previously validated to be a non-nil and valid blossom URL.
	Mirror slice[func(r Request, url *url.URL) *blossom.Error]

	// Media is invoked when processing the HEAD /media and before processing every PUT /media request.
	Media slice[func(r Request, hints UploadHints) *blossom.Error]

	// Report is invoked before processing a PUT /report request.
	Report slice[func(r Request, report Report) *blossom.Error]
}

// OnHooks defines functions invoked after specific blossom events occur.
// These hooks customize how the server reacts to requests such as upload or delete.
// Each function is called only after the corresponding input has passed all RejectHooks (if any).
//
// OnHooks are typically used to implement custom processing, persistence,
// logging, authorization, or other side effects in response to relay activity.
type OnHooks struct {
	// FetchBlob handles the core logic for GET /<sha256>.<ext> as per BUD-01.
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/01.md
	FetchBlob func(r Request, hash blossom.Hash, ext string) (blossom.Blob, *blossom.Error)

	// FetchMeta handles the core logic for HEAD /<sha256>.<ext> as per BUD-01.
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/01.md
	FetchMeta func(r Request, hash blossom.Hash, ext string) (mime string, size int64, err *blossom.Error)

	// Delete handles the core logic for DELETE /<sha256> as per BUD-02.
	// This hook is optional. If not specified, the endpoint will return the http status code 501 (Not Implemented).
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/02.md
	Delete func(r Request, hash blossom.Hash) *blossom.Error

	// Upload handles the core logic for PUT /upload as per BUD-02.
	// If the returned blob descriptor has an empty URL, the server will automatically derive it from the
	// baseURL, the hash and the type of the blob.
	// This hook is optional. If not specified, the endpoint will return the http status code 501 (Not Implemented).
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/02.md
	Upload func(r Request, hints UploadHints, data io.Reader) (blossom.BlobDescriptor, *blossom.Error)

	// Mirror handles the core logic for PUT /mirror as per BUD-04.
	// The url has been previously validated to be a non-nil and valid blossom URL.
	// If the returned blob descriptor has an empty URL, the server will automatically derive it from the
	// baseURL, the hash and the type of the blob.
	// This hook is optional. If not specified, the endpoint will return the http status code 501 (Not Implemented).
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/04.md
	Mirror func(r Request, url *url.URL) (blossom.BlobDescriptor, *blossom.Error)

	// Media handles the core logic for PUT /media as per BUD-05.
	// If the returned blob descriptor has an empty URL, the server will automatically derive it from the
	// baseURL, the hash and the type of the blob.
	// This hook is optional. If not specified, the endpoint will return the http status code 501 (Not Implemented).
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/05.md
	Media func(r Request, hints UploadHints, data io.Reader) (blossom.BlobDescriptor, *blossom.Error)

	// Report handles the core logic for PUT /report as per BUD-09.
	// This hook is optional. If not specified, the endpoint will return the http status code 501 (Not Implemented).
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/09.md
	Report func(r Request, report Report) *blossom.Error
}

func NewOnHooks() OnHooks {
	return OnHooks{
		FetchBlob: defaultFetchBlob,
		FetchMeta: defaultFetchMeta,
	}
}

func defaultFetchBlob(r Request, hash blossom.Hash, ext string) (blossom.Blob, *blossom.Error) {
	slog.Info("received GET request", "hash", hash.Hex(), "ext", ext, "ip", r.IP().Group())
	return nil, &blossom.Error{Code: http.StatusNotFound, Reason: "The FetchBlob hook is not configured"}
}

func defaultFetchMeta(r Request, hash blossom.Hash, ext string) (mime string, size int64, err *blossom.Error) {
	slog.Info("received HEAD request", "hash", hash.Hex(), "ext", ext, "ip", r.IP().Group())
	return "", 0, &blossom.Error{Code: http.StatusNotFound, Reason: "The FetchMeta hook is not configured"}
}

// Slice is an internal type used to simplify registration of hooks.
type slice[T any] []T

// Append adds hooks to the end of the slice, in the provided order.
func (s *slice[T]) Append(hooks ...T) {
	*s = append(*s, hooks...)
}

// Prepend adds hooks to the start of the slice, in the provided order.
func (s *slice[T]) Prepend(hooks ...T) {
	*s = append(hooks, *s...)
}

// Clear resets the slice, removing all registered hooks.
func (s *slice[T]) Clear() {
	*s = nil
}
