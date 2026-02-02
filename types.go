package blossy

import (
	"net/http"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/blossom"
)

// BlobDelivery represents how a blob should be delivered to the client.
// Use [Serve] to serve a [blossom.Blob] directly to the client or [Redirect] to redirect the client to another URL.
type BlobDelivery interface {
	sealBlob() // unexported method seals the interface
}

// MetaDelivery represents how blob metadata should be delivered to the client.
// Use [Found] to return the metadata directly, or [Redirect] to redirect the client to another URL.
type MetaDelivery interface {
	sealMeta() // unexported method seals the interface
}

// redirect can be used as both [BlobDelivery] and [MetaDelivery].
type redirect interface {
	BlobDelivery
	MetaDelivery
}

type servedBlob struct {
	blossom.Blob
}

func (servedBlob) sealBlob() {}

type redirectedBlob struct {
	url  string
	code int
}

func (redirectedBlob) sealBlob() {}
func (redirectedBlob) sealMeta() {}

type foundBlob struct {
	mime string
	size int64
}

func (foundBlob) sealMeta() {}

// Serve creates a BlobDelivery that serves the blob directly to the client.
func Serve(blob blossom.Blob) BlobDelivery {
	return servedBlob{blob}
}

// Found creates a MetaDelivery that returns the blob metadata directly to the client.
func Found(mime string, size int64) MetaDelivery {
	return foundBlob{mime: mime, size: size}
}

// Redirect creates a response that redirects the client to the given URL.
// It can be used as both [BlobDelivery] and [MetaDelivery].
// Common status codes are http.StatusFound (302) or http.StatusMovedPermanently (301).
func Redirect(url string, code int) redirect {
	if code == 0 {
		code = http.StatusFound
	}
	return redirectedBlob{url: url, code: code}
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
