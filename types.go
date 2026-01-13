package blossy

import (
	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/blossom"
)

// BlobDescriptor represent a description of a blossom blob.
// Learn more here: https://github.com/hzrd149/blossom/blob/master/buds/02.md#blob-descriptor
type BlobDescriptor struct {
	URL      string `json:"url"`    // Depends on the [Server] base URL
	SHA256   string `json:"sha256"` // Hex-encoded SHA256 hash
	Size     int64  `json:"size"`
	Type     string `json:"type"`
	Uploaded int64  `json:"uploaded"` // Unix timestamp
}

// UploadHints contains hints about the uploaded blob as reported by the client.
// They can be used for rejection or optimization purposes, but they must not be trusted
// as they can be easily spoofed.
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

// ReportedBlob represents a blob that was reported for the provided reason.
type ReportedBlob struct {
	Hash   blossom.Hash
	Reason string
}

// Report is a normalized form of a NIP-56 report received in the /report endpoint.
// Learn more here: https://github.com/nostr-protocol/nips/blob/master/56.md
type Report struct {
	Pubkey  string
	Blobs   []ReportedBlob
	Content string

	Raw *nostr.Event
}

// Hashes returns the list of blob hashes in the report.
func (r Report) Hashes() []blossom.Hash {
	hashes := make([]blossom.Hash, len(r.Blobs))
	for i := range r.Blobs {
		hashes[i] = r.Blobs[i].Hash
	}
	return hashes
}
