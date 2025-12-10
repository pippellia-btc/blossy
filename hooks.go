package blossom

// Hooks of the blossom server, that the user of this framework can configure.
type Hooks struct {
	Reject RejectHooks
	On     OnHooks
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

	// Has is invoked before processing a HEAD /<sha256>.<ext> request.
	Has slice[func(request Request, hash, ext string) *Error]
}

type OnHooks struct {
	// Get handles the core logic for GET /<sha256>.<ext> as per BUD-01.
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/01.md
	Get func(request Request, hash, ext string) (blob Blob, err *Error)

	// Has handles the core logic for HEAD /<sha256>.<ext> as per BUD-01.
	// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/01.md
	Has func(request Request, hash, ext string) (mime string, size int64, err *Error)
}
