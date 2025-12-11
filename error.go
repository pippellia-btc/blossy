package blossom

import (
	"fmt"
	"net/http"
)

// Error represent an http error with the specified code and reason.
// If the reason is not empty, it is written in the "X-Reason" header as per BUD-01.
type Error struct {
	Code   int
	Reason string
}

func (e Error) Error() string {
	return e.Reason
}

func (e Error) String() string {
	return fmt.Sprintf("code: %d, reason: %s", e.Code, e.Reason)
}

func (e *Error) Is(target error) bool {
	if e == nil {
		return target == nil
	}

	err, ok := target.(Error)
	if !ok {
		return false
	}
	return e.Code == err.Code && e.Reason == err.Reason
}

// Write the error to the http response. If the reason is non-empty,
// it writes it to the "X-Reason" header as per BUD-01.
func (e Error) Write(w http.ResponseWriter) {
	if e.Reason != "" {
		w.Header().Set("X-Reason", e.Reason)
	}
	http.Error(w, e.Reason, e.Code)
}
