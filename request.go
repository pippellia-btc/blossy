package blossy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/blossom"
)

// Request represents metadata about an incoming HTTP request.
// It provides access to identifying, network, authentication, and
// contextual information, as well as the underlying raw http.Request.
type Request interface {
	// ID is the unique identified of the request, useful for logging or tracking.
	ID() int64

	// IP address where the request come from.
	// For rate-limiting purposes you should use [IP.Group] or [IP.GroupPrefix]
	// as a normalized representation of the IP.
	IP() IP

	// Pubkey that signed a valid authorization event if present, otherwise "".
	Pubkey() string

	// IsAuthed returns whether the request has a valid authorization event.
	// It's a shorter version of Request.Pubkey() != "".
	IsAuthed() bool

	// Context returns the context of the underlying [http.Request].
	Context() context.Context

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
func (r request) IP() IP                   { return r.ip }
func (r request) Pubkey() string           { return r.pubkey }
func (r request) IsAuthed() bool           { return r.pubkey != "" }
func (r request) Context() context.Context { return r.raw.Context() }
func (r request) Raw() *http.Request       { return r.raw }

func parseFetch(r *http.Request) (request, blossom.Hash, string, *blossom.Error) {
	hash, ext, err := ParseHash(r.URL.Path)
	if err != nil {
		return request{}, blossom.Hash{}, "", &blossom.Error{Code: http.StatusBadRequest, Reason: err.Error()}
	}

	pubkey, err := parsePubkey(r.Header, VerbGet, hash)
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return request{}, blossom.Hash{}, "", &blossom.Error{Code: http.StatusUnauthorized, Reason: err.Error()}
	}

	req := request{
		ip:     GetIP(r),
		pubkey: pubkey,
		raw:    r,
	}
	return req, hash, ext, nil
}

func parseDelete(r *http.Request) (request, blossom.Hash, *blossom.Error) {
	hash, _, err := ParseHash(r.URL.Path)
	if err != nil {
		return request{}, blossom.Hash{}, &blossom.Error{Code: http.StatusBadRequest, Reason: err.Error()}
	}

	pubkey, err := parsePubkey(r.Header, VerbDelete, hash)
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return request{}, blossom.Hash{}, &blossom.Error{Code: http.StatusUnauthorized, Reason: err.Error()}
	}

	req := request{
		ip:     GetIP(r),
		pubkey: pubkey,
		raw:    r,
	}
	return req, hash, nil
}

func parseUpload(r *http.Request) (request, UploadHints, io.ReadCloser, *blossom.Error) {
	// In the future I want to pass the hash of the body.
	// Now there is no point since the auth scheme is broken anyway.
	// See https://github.com/hzrd149/blossom/pull/87
	pubkey, err := parsePubkey(r.Header, VerbUpload, blossom.Hash{})
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return request{}, UploadHints{}, nil, &blossom.Error{Code: http.StatusUnauthorized, Reason: err.Error()}
	}

	hints := UploadHints{
		Type: r.Header.Get("Content-Type"),
		Size: -1, // default to unknown
	}

	if cl := r.Header.Get("Content-Length"); cl != "" {
		size, err := strconv.ParseInt(cl, 10, 64)
		if err == nil {
			hints.Size = size
		}
	}

	req := request{
		ip:     GetIP(r),
		pubkey: pubkey,
		raw:    r,
	}
	return req, hints, r.Body, nil
}

func parseUploadCheck(r *http.Request) (request, UploadHints, *blossom.Error) {
	sha256 := r.Header.Get("X-SHA-256")
	if sha256 == "" {
		return request{}, UploadHints{}, &blossom.Error{Code: http.StatusBadRequest, Reason: "'X-SHA-256' header is missing or empty"}
	}
	hash, err := blossom.ParseHash(sha256)
	if err != nil {
		return request{}, UploadHints{}, &blossom.Error{Code: http.StatusBadRequest, Reason: "'X-SHA-256' header is invalid: " + err.Error()}
	}

	cl := r.Header.Get("X-Content-Length")
	if cl == "" {
		return request{}, UploadHints{}, &blossom.Error{Code: http.StatusBadRequest, Reason: "'X-Content-Length' header is missing or empty"}
	}
	size, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return request{}, UploadHints{}, &blossom.Error{Code: http.StatusBadRequest, Reason: "'X-Content-Length' header is invalid: " + err.Error()}
	}

	mime := r.Header.Get("X-Content-Type")
	if mime == "" {
		return request{}, UploadHints{}, &blossom.Error{Code: http.StatusBadRequest, Reason: "'X-Content-Type' header is missing or empty"}
	}

	pubkey, err := parsePubkey(r.Header, VerbUpload, hash)
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return request{}, UploadHints{}, &blossom.Error{Code: http.StatusUnauthorized, Reason: err.Error()}
	}

	req := request{
		ip:     GetIP(r),
		pubkey: pubkey,
		raw:    r,
	}
	hints := UploadHints{
		Hash: hash,
		Type: mime,
		Size: size,
	}
	return req, hints, nil
}

func parseMirror(r *http.Request) (request, *url.URL, *blossom.Error) {
	data, rerr := ReadNoMore(r.Body, 512)
	if rerr != nil {
		return request{}, nil, rerr
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	hash := sha256.Sum256(data)

	var payload struct {
		URL string `json:"url"`
	}

	if err := dec.Decode(&payload); err != nil {
		return request{}, nil, &blossom.Error{Code: http.StatusBadRequest, Reason: "failed to parse JSON body: " + err.Error()}
	}

	url, err := url.Parse(payload.URL)
	if err != nil {
		return request{}, nil, &blossom.Error{Code: http.StatusBadRequest, Reason: "failed to parse URL: " + err.Error()}
	}

	if err := ValidateBlossomURL(url); err != nil {
		return request{}, nil, &blossom.Error{Code: http.StatusBadRequest, Reason: "invalid blossom URL: " + err.Error()}
	}

	pubkey, err := parsePubkey(r.Header, VerbUpload, hash)
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return request{}, nil, &blossom.Error{Code: http.StatusUnauthorized, Reason: err.Error()}
	}

	req := request{
		ip:     GetIP(r),
		pubkey: pubkey,
		raw:    r,
	}
	return req, url, nil
}

func parseReport(r *http.Request) (request, Report, *blossom.Error) {
	data, rerr := ReadNoMore(r.Body, 100_000) // ~100 KB
	if rerr != nil {
		return request{}, Report{}, rerr
	}

	event := &nostr.Event{}
	dec := json.NewDecoder(bytes.NewReader(data))

	if err := dec.Decode(&event); err != nil {
		return request{}, Report{}, &blossom.Error{Code: http.StatusBadRequest, Reason: "failed to parse JSON body: " + err.Error()}
	}

	report, err := parseReportEvent(event)
	if err != nil {
		return request{}, Report{}, &blossom.Error{Code: http.StatusBadRequest, Reason: err.Error()}
	}

	req := request{
		ip:  GetIP(r),
		raw: r,
	}
	return req, report, nil
}

// ParseHash extracts the SHA256 hash and the optional extension from URL path.
func ParseHash(path string) (hash blossom.Hash, ext string, err error) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, ".", 2) // separate hash from extension

	hash, err = blossom.ParseHash(parts[0])
	if err != nil {
		return blossom.Hash{}, "", err
	}

	if len(parts) > 1 {
		ext = parts[1]
	}
	return hash, ext, nil
}

// ValidateBlossomURL checks whether the provided URL contains a valid blossom hash in its path.
func ValidateBlossomURL(url *url.URL) error {
	path := strings.TrimPrefix(url.Path, "/")
	parts := strings.SplitN(path, ".", 2) // separate hash from extension
	_, err := blossom.ParseHash(parts[0])
	return err
}

// ReadNoMore reads no more than `limit` bytes from the reader.
// It returns an error if the reader has more bytes than `limit` to be read.
func ReadNoMore(r io.Reader, limit int) ([]byte, *blossom.Error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(limit)))
	if err != nil {
		return nil, &blossom.Error{Code: http.StatusBadRequest, Reason: "failed to read body: " + err.Error()}
	}
	if len(data) >= limit {
		return nil, &blossom.Error{Code: http.StatusRequestEntityTooLarge, Reason: "body too large"}
	}
	return data, nil
}

// ParseReportEvent parses a [Report] from the underlying nostr event.
func parseReportEvent(event *nostr.Event) (Report, error) {
	if event.Kind != nostr.KindReporting {
		return Report{}, errors.New("report event must be a kind 1984")
	}

	report := Report{
		Pubkey:  event.PubKey,
		Content: event.Content,
		Raw:     event,
	}

	for _, tag := range event.Tags {
		if len(tag) < 3 || tag[0] != "x" {
			continue
		}

		hash, err := blossom.ParseHash(tag[1])
		if err != nil {
			return Report{}, fmt.Errorf("invalid \"x\" tag in report event: %w", err)
		}

		report.Blobs = append(report.Blobs,
			ReportedBlob{Hash: hash, Reason: tag[2]},
		)
	}

	if err := verify(event); err != nil {
		return Report{}, fmt.Errorf("invalid report event: %w", err)
	}
	return report, nil
}
