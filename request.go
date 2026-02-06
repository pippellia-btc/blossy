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
		return request{}, blossom.Hash{}, "", blossom.ErrBadRequest(err.Error())
	}

	pubkey, err := parsePubkey(r.Header, VerbGet, hash)
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return request{}, blossom.Hash{}, "", blossom.ErrUnauthorized(err.Error())
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
		return request{}, blossom.Hash{}, blossom.ErrBadRequest(err.Error())
	}

	pubkey, err := parsePubkey(r.Header, VerbDelete, hash)
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return request{}, blossom.Hash{}, blossom.ErrUnauthorized(err.Error())
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
		return request{}, UploadHints{}, nil, blossom.ErrUnauthorized(err.Error())
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

	if sha := r.Header.Get("Content-Digest"); sha != "" {
		hash, err := blossom.ParseHash(sha)
		if err != nil {
			return request{}, UploadHints{}, nil, blossom.ErrBadRequest("'Content-Digest' header is invalid: " + err.Error())
		}
		hints.Hash = hash
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
		return request{}, UploadHints{}, blossom.ErrBadRequest("'X-SHA-256' header is missing or empty")
	}
	hash, err := blossom.ParseHash(sha256)
	if err != nil {
		return request{}, UploadHints{}, blossom.ErrBadRequest("'X-SHA-256' header is invalid: " + err.Error())
	}

	cl := r.Header.Get("X-Content-Length")
	if cl == "" {
		return request{}, UploadHints{}, blossom.ErrBadRequest("'X-Content-Length' header is missing or empty")
	}
	size, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return request{}, UploadHints{}, blossom.ErrBadRequest("'X-Content-Length' header is invalid: " + err.Error())
	}

	mime := r.Header.Get("X-Content-Type")
	if mime == "" {
		return request{}, UploadHints{}, blossom.ErrBadRequest("'X-Content-Type' header is missing or empty")
	}

	pubkey, err := parsePubkey(r.Header, VerbUpload, hash)
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return request{}, UploadHints{}, blossom.ErrUnauthorized(err.Error())
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
		return request{}, nil, blossom.ErrBadRequest("failed to parse JSON body: " + err.Error())
	}

	url, err := url.Parse(payload.URL)
	if err != nil {
		return request{}, nil, blossom.ErrBadRequest("failed to parse URL: " + err.Error())
	}

	if err := ValidateBlossomURL(url); err != nil {
		return request{}, nil, blossom.ErrBadRequest("invalid blossom URL: " + err.Error())
	}

	pubkey, err := parsePubkey(r.Header, VerbUpload, hash)
	if err != nil && !errors.Is(err, ErrAuthMissingHeader) {
		return request{}, nil, blossom.ErrUnauthorized(err.Error())
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
		return request{}, Report{}, blossom.ErrBadRequest("failed to parse JSON body: " + err.Error())
	}

	report, err := parseReportEvent(event)
	if err != nil {
		return request{}, Report{}, blossom.ErrBadRequest(err.Error())
	}

	req := request{
		ip:  GetIP(r),
		raw: r,
	}
	return req, report, nil
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
