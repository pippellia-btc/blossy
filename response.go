package blossom

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Blob is the fundamental data structure of blossom, returned by the Get hook.
type Blob struct {
	Data io.ReadCloser
	MIME string
	Size int64
}

// Write the Blob data to the provided http.ResponseWriter.
// It sets the Content-Type and Content-Length headers automatically.
func (blob Blob) Write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", blob.MIME)
	w.Header().Set("Content-Length", strconv.FormatInt(blob.Size, 10))

	defer blob.Data.Close()
	written, err := io.Copy(w, blob.Data)
	if err != nil {
		return fmt.Errorf("failed to copy blob data to response: %w", err)
	}

	if written != blob.Size {
		return fmt.Errorf("copied size mismatch: expected %d, wrote %d", blob.Size, written)
	}
	return nil
}
