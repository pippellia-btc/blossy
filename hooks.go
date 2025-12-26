package blossy

import (
	"io"
	"log/slog"

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

type RejectHooks struct {
	// FetchBlob is invoked before processing a GET /<hash>.<ext> request.
	// If any of the hooks returns a non-nil error, the request is rejected.
	FetchBlob slice[func(r Request, hash blossom.Hash, ext string) *blossom.Error]

	// FetchMeta is invoked before processing a HEAD /<hash>.<ext> request.
	// If any of the hooks returns a non-nil error, the request is rejected.
	FetchMeta slice[func(r Request, hash blossom.Hash, ext string) *blossom.Error]

	// Upload is invoked when processing the HEAD /upload and before processing
	// every PUT /upload request. If any of the hooks returns a non-nil error, the request is rejected.
	Upload slice[func(r Request, hints UploadHints) *blossom.Error]
}

type OnHooks struct {
	// FetchBlob handles the core logic for GET /<sha256>.<ext> as per BUD-01.
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/01.md
	FetchBlob func(r Request, hash blossom.Hash, ext string) (io.ReadSeekCloser, *blossom.Error)

	// FetchMeta handles the core logic for HEAD /<sha256>.<ext> as per BUD-01.
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/01.md
	FetchMeta func(r Request, hash blossom.Hash, ext string) (mime string, size int64, err *blossom.Error)

	// Upload handles the core logic for PUT /upload as per BUD-01.
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/02.md
	Upload func(r Request, hints UploadHints, body io.ReadCloser) (blossom.BlobMeta, *blossom.Error)
}

func NewOnHooks() OnHooks {
	return OnHooks{
		FetchBlob: logFetchBlob,
		FetchMeta: logFetchMeta,
	}
}

func logFetchBlob(r Request, h blossom.Hash, ext string) (io.ReadSeekCloser, *blossom.Error) {
	slog.Info("received GET request", "hash", h, "extention", ext, "ip", r.IP().Group())
	return nil, &blossom.Error{Code: 404, Reason: "The FetchBlob hook is not configured"}
}

func logFetchMeta(r Request, h blossom.Hash, ext string) (mime string, size int64, err *blossom.Error) {
	slog.Info("received HEAD request", "hash", h, "extention", ext, "ip", r.IP().Group())
	return "", 0, &blossom.Error{Code: 404, Reason: "The FetchMeta hook is not configured"}
}
