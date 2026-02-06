package blossy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/blossom"
	"github.com/pippellia-btc/blossy/auth"
	"github.com/pippellia-btc/blossy/utils"
)

// Request represents metadata about an incoming HTTP request.
// It provides access to identifying, network, authentication, and
// contextual information, as well as the underlying raw http.Request.
type Request interface {
	// ID is the unique identifier of the request, useful for logging or tracking.
	ID() int64

	// IP address where the request comes from.
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

func (s *Server) parseFetch(r *http.Request) (request, blossom.Hash, string, *blossom.Error) {
	hash, ext, err := utils.ParseHashExt(r.URL.Path)
	if err != nil {
		return request{}, blossom.Hash{}, "", blossom.ErrBadRequest(err.Error())
	}

	pubkey, err := auth.Authenticate(r, s.Sys.hostname, &hash)
	if err != nil {
		return request{}, blossom.Hash{}, "", blossom.ErrUnauthorized(err.Error())
	}

	req := request{
		id:     s.nextRequest.Add(1),
		ip:     GetIP(r),
		pubkey: pubkey,
		raw:    r,
	}
	return req, hash, ext, nil
}

func (s *Server) parseDelete(r *http.Request) (request, blossom.Hash, *blossom.Error) {
	hash, _, err := utils.ParseHashExt(r.URL.Path)
	if err != nil {
		return request{}, blossom.Hash{}, blossom.ErrBadRequest(err.Error())
	}

	pubkey, err := auth.Authenticate(r, s.Sys.hostname, &hash)
	if err != nil {
		return request{}, blossom.Hash{}, blossom.ErrUnauthorized(err.Error())
	}

	req := request{
		id:     s.nextRequest.Add(1),
		ip:     GetIP(r),
		pubkey: pubkey,
		raw:    r,
	}
	return req, hash, nil
}

func (s *Server) parseUpload(r *http.Request) (request, UploadHints, io.ReadCloser, *blossom.Error) {
	hints := UploadHints{
		Type: r.Header.Get("Content-Type"),
		Size: -1, // stands for unknown
	}

	if cl := r.Header.Get("Content-Length"); cl != "" {
		size, err := strconv.ParseInt(cl, 10, 64)
		if err != nil {
			return request{}, UploadHints{}, nil, blossom.ErrBadRequest("'Content-Length' header is invalid: " + err.Error())
		}
		if size <= 0 {
			return request{}, UploadHints{}, nil, blossom.ErrBadRequest("'Content-Length' header is invalid: size must be greater than 0")
		}
		hints.Size = size
	}

	if sha := r.Header.Get("Content-Digest"); sha != "" {
		hash, err := blossom.ParseHash(sha)
		if err != nil {
			return request{}, UploadHints{}, nil, blossom.ErrBadRequest("'Content-Digest' header is invalid: " + err.Error())
		}
		hints.Hash = &hash
	}

	pubkey, err := auth.Authenticate(r, s.Sys.hostname, hints.Hash)
	if errors.Is(err, auth.ErrMissingHash) {
		return request{}, UploadHints{}, nil, blossom.ErrBadRequest("'Content-Digest' header is missing or empty")
	}
	if err != nil {
		return request{}, UploadHints{}, nil, blossom.ErrUnauthorized(err.Error())
	}

	req := request{
		id:     s.nextRequest.Add(1),
		ip:     GetIP(r),
		pubkey: pubkey,
		raw:    r,
	}
	return req, hints, r.Body, nil
}

func (s *Server) parseUploadCheck(r *http.Request) (request, UploadHints, *blossom.Error) {
	ct := r.Header.Get("X-Content-Type")
	if ct == "" {
		return request{}, UploadHints{}, blossom.ErrBadRequest("'X-Content-Type' header is missing or empty")
	}

	cl := r.Header.Get("X-Content-Length")
	if cl == "" {
		return request{}, UploadHints{}, blossom.ErrBadRequest("'X-Content-Length' header is missing or empty")
	}
	size, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return request{}, UploadHints{}, blossom.ErrBadRequest("'X-Content-Length' header is invalid: " + err.Error())
	}
	if size <= 0 {
		return request{}, UploadHints{}, blossom.ErrBadRequest("'X-Content-Length' header is invalid: size must be greater than 0")
	}

	sha256 := r.Header.Get("X-SHA-256")
	if sha256 == "" {
		return request{}, UploadHints{}, blossom.ErrBadRequest("'X-SHA-256' header is missing or empty")
	}
	hash, err := blossom.ParseHash(sha256)
	if err != nil {
		return request{}, UploadHints{}, blossom.ErrBadRequest("'X-SHA-256' header is invalid: " + err.Error())
	}

	hints := UploadHints{
		Hash: &hash,
		Type: ct,
		Size: size,
	}

	pubkey, err := auth.Authenticate(r, s.Sys.hostname, hints.Hash)
	if err != nil {
		return request{}, UploadHints{}, blossom.ErrUnauthorized(err.Error())
	}

	req := request{
		id:     s.nextRequest.Add(1),
		ip:     GetIP(r),
		pubkey: pubkey,
		raw:    r,
	}
	return req, hints, nil
}

func (s *Server) parseMirror(r *http.Request) (request, *url.URL, *blossom.Error) {
	body, rerr := utils.ReadNoMore(r.Body, 512)
	if rerr != nil {
		return request{}, nil, rerr
	}

	var payload struct {
		URL string `json:"url"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return request{}, nil, blossom.ErrBadRequest("failed to parse JSON body: " + err.Error())
	}

	url, err := url.Parse(payload.URL)
	if err != nil {
		return request{}, nil, blossom.ErrBadRequest("failed to parse URL: " + err.Error())
	}
	if url.Host == "" || url.Scheme != "https" {
		return request{}, nil, blossom.ErrBadRequest("URL is invalid: must be a valid HTTPS URL")
	}

	hash, _, err := utils.ParseHashExt(url.Path)
	if err != nil {
		return request{}, nil, blossom.ErrBadRequest("invalid blossom URL: " + err.Error())
	}

	pubkey, err := auth.Authenticate(r, s.Sys.hostname, &hash)
	if err != nil {
		return request{}, nil, blossom.ErrUnauthorized(err.Error())
	}

	req := request{
		id:     s.nextRequest.Add(1),
		ip:     GetIP(r),
		pubkey: pubkey,
		raw:    r,
	}
	return req, url, nil
}

func (s *Server) parseReport(r *http.Request) (request, Report, *blossom.Error) {
	body, rerr := utils.ReadNoMore(r.Body, 100_000) // ~100 KB
	if rerr != nil {
		return request{}, Report{}, rerr
	}

	event := &nostr.Event{}
	if err := json.Unmarshal(body, event); err != nil {
		return request{}, Report{}, blossom.ErrBadRequest("failed to parse JSON body: " + err.Error())
	}

	report, err := parseReportEvent(event)
	if err != nil {
		return request{}, Report{}, blossom.ErrBadRequest(err.Error())
	}

	req := request{
		id:  s.nextRequest.Add(1),
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

		report.Blobs = append(report.Blobs, ReportedBlob{Hash: hash, Reason: tag[2]})
	}

	if len(report.Blobs) == 0 {
		return Report{}, errors.New("invalid report event: no blobs reported")
	}

	if !event.CheckID() {
		return Report{}, errors.New("invalid report event: event ID is not valid")
	}
	match, err := event.CheckSignature()
	if err != nil {
		return Report{}, fmt.Errorf("invalid report event: %w", err)
	}
	if !match {
		return Report{}, errors.New("invalid report event: event signature is not valid")
	}

	return report, nil
}
