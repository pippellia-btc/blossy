package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/blossom"
	"github.com/pippellia-btc/blossy/utils"
)

// Action represent the action that the request is trying to perform.
type Action string

var (
	ActionGet    Action = "get"
	ActionUpload Action = "upload"
	ActionList   Action = "list"
	ActionDelete Action = "delete"

	validActions = []Action{ActionGet, ActionUpload, ActionList, ActionDelete}
)

var (
	ErrMissingHeader = errors.New("missing 'Authorization' header")
	ErrInvalidScheme = errors.New("authorization scheme must be 'Nostr <base64_event>'")
	ErrInvalidBase64 = errors.New("failed to decode base64 event payload")
	ErrInvalidJSON   = errors.New("invalid event json")
	ErrMissingHash   = errors.New("auth event has 'x' tags but no hash was provided to match against")
)

// Authenticate validates the authorization event against the provided hostname and hash,
// and returns the pubkey of the signed event if valid.
// If the "Authorization" header is missing, it returns an empty pubkey.
// If the "Authorization" header is present but the event is invalid, it returns an error.
//
// It accepts a nil hash to distinguish between the zero hash and no hash.
// The distinction is important because a GET might require the hash 000...000,
// while an upload might not have a hash at all in the Content-Digest header.
func Authenticate(r *http.Request, hostname string, hash *blossom.Hash) (pubkey string, err error) {
	event, err := ExtractEvent(r)
	if errors.Is(err, ErrMissingHeader) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	if !event.CheckID() {
		return "", errors.New("auth failed: invalid event ID")
	}
	match, err := event.CheckSignature()
	if err != nil {
		return "", fmt.Errorf("auth failed: invalid event signature: %w", err)
	}
	if !match {
		return "", errors.New("auth failed: invalid event signature")
	}

	action, err := impliedAction(r)
	if err != nil {
		return "", fmt.Errorf("auth failed: %w", err)
	}

	switch event.Kind {
	case KindBlossomAuth:
		auth, err := ParseBlossomAuth(event)
		if err != nil {
			return "", fmt.Errorf("auth failed: %w", err)
		}
		if err := auth.Validate(action, hash, hostname); err != nil {
			return "", fmt.Errorf("auth failed: %w", err)
		}
		return auth.Pubkey, nil

	// TODO: Add NWT support

	default:
		return "", fmt.Errorf("auth failed: unsupported event kind: %d", event.Kind)
	}
}

// ExtractEvent extracts the authentication event from the "Authorization" request header,
// encoded as base64 string. It returns an error if the header is missing, invalid,
// or the base64 decoding fails.
func ExtractEvent(r *http.Request) (*nostr.Event, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return nil, ErrMissingHeader
	}

	parts := strings.Split(auth, " ")
	if len(parts) != 2 {
		return nil, ErrInvalidScheme
	}
	if parts[0] != "Nostr" {
		return nil, ErrInvalidScheme
	}

	bytes, err := utils.DecodeBase64(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidBase64, err)
	}

	event := &nostr.Event{}
	if err := json.Unmarshal(bytes, event); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	return event, nil
}

// ImpliedAction returns the implied [Action] for the given request.
func impliedAction(r *http.Request) (Action, error) {
	p := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	case p == "upload" || p == "media" || p == "mirror":
		return ActionUpload, nil

	case strings.HasPrefix(p, "list"):
		return ActionList, nil

	case r.Method == http.MethodGet || r.Method == http.MethodHead:
		return ActionGet, nil

	case r.Method == http.MethodDelete:
		return ActionDelete, nil

	default:
		return "", fmt.Errorf("this request doesn't have an implied action: method=%s path=%s", r.Method, p)
	}
}
