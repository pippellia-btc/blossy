package blossy

import "github.com/pippellia-btc/blossom"

// UploadHints contains hints about the uploaded blob as reported by the client.
// They can be used for rejection or optimization purposes, but they must not be trusted
// as they come from the client and can be easily spoofed.
type UploadHints struct {
	// Hash is the sha256 hash of the uploaded blob as reported by the client.
	// If unknown, it will be the zero value.
	Hash blossom.Hash

	// Type is the content type of the uploaded blob.
	// If unknown, it will be an empty string.
	Type string

	// Size is the size in bytes of the uploaded blob.
	// If unknown, it will be -1.
	Size int64
}

type BlobDescriptor struct {
	URL      string `json:"url"`
	SHA256   string `json:"sha256"` // Hex-encoded SHA256 hash
	Size     int64  `json:"size"`
	Type     string `json:"type"`
	Uploaded int64  `json:"uploaded"` // Unix timestamp
}
