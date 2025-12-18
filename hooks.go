package blossy

import "log/slog"

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
	// Get is invoked before processing a GET /<sha256>.<ext> request.
	Get slice[func(request Request, hash, ext string) *Error]

	// Check is invoked before processing a HEAD /<sha256>.<ext> request.
	Check slice[func(request Request, hash, ext string) *Error]
}

type OnHooks struct {
	// Get handles the core logic for GET /<sha256>.<ext> as per BUD-01.
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/01.md
	Get func(request Request, hash, ext string) (blob Blob, err *Error)

	// Check handles the core logic for HEAD /<sha256>.<ext> as per BUD-01.
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/01.md
	Check func(request Request, hash, ext string) (mime string, size int64, err *Error)
}

func NewOnHooks() OnHooks {
	return OnHooks{
		Get:   logGet,
		Check: logCheck,
	}
}

func logGet(request Request, hash, ext string) (Blob, *Error) {
	slog.Info("received GET request", "hash", hash, "extention", ext, "ip", request.IP().Group())
	return Blob{}, &Error{Code: 404, Reason: "The GET hook is not configured"}
}

func logCheck(request Request, hash, ext string) (mime string, size int64, err *Error) {
	slog.Info("received GET request", "hash", hash, "extention", ext, "ip", request.IP().Group())
	return "", 0, &Error{Code: 404, Reason: "The GET hook is not configured"}
}
