package blossy

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/blossom"
)

const KindAuth = 24242

type Verb string

var (
	VerbGet    Verb = "get"
	VerbUpload Verb = "upload"
	VerbList   Verb = "list"
	VerbDelete Verb = "delete"
)

var (
	ErrAuthMissingHeader = errors.New("missing 'Authorization' header")
	ErrAuthInvalidScheme = errors.New("authorization scheme must be 'Nostr <base64_event>'")
	ErrAuthInvalidBase64 = errors.New("failed to decode base64 event payload")

	ErrInvalidEventJSON = errors.New("invalid event json")
	ErrInvalidEventID   = errors.New("invalid event ID")
	ErrInvalidEventSig  = errors.New("invalid event signature")

	ErrAuthInvalidKind      = errors.New("kind must be 24242")
	ErrAuthInvalidTimestamp = errors.New("created_at is in the future")

	ErrAuthMissingXTag          = errors.New("'x' tag is missing")
	ErrAuthInvalidXTag          = errors.New("'x' tag is invalid")
	ErrAuthMissingVerbTag       = errors.New("'t' tag is missing")
	ErrAuthInvalidVerbTag       = errors.New("'t' tag is invalid")
	ErrAuthMissingExpirationTag = errors.New("'expiration' tag is missing")
	ErrAuthInvalidExpirationTag = errors.New("'expiration' tag is invalid")
)

// parsePubkey from the authentication event in the header.
// If the 'Authorization' header is not present, it returns [ErrAuthMissingHeader].
// If the 'Authorization' header contains an in invalid authentication event, it returns the specific error.
func parsePubkey(header http.Header, verb Verb, hash blossom.Hash) (string, error) {
	event, err := parseAuth(header)
	if err != nil {
		return "", err
	}

	if err := validateAuth(event, verb, hash); err != nil {
		return "", err
	}
	return event.PubKey, nil
}

// ValidateAuth validates the authentication event against the expected verb and hash.
// This (correct) implementation of the protocol is not secure. See https://github.com/hzrd149/blossom/pull/87
func validateAuth(event *nostr.Event, verb Verb, hash blossom.Hash) error {
	if event.Kind != KindAuth {
		return ErrAuthInvalidKind
	}

	now := time.Now().Unix()
	if int64(event.CreatedAt) > now {
		return ErrAuthInvalidTimestamp
	}

	expTag, found := firstTag(event, "expiration")
	if !found {
		return ErrAuthMissingExpirationTag
	}
	expiration, err := strconv.ParseInt(expTag, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAuthInvalidExpirationTag, err)
	}
	if expiration <= now {
		return fmt.Errorf("%w: expiration is in the past", ErrAuthInvalidExpirationTag)
	}

	tTag, found := firstTag(event, "t")
	if !found {
		return ErrAuthMissingVerbTag
	}
	if Verb(tTag) != verb {
		return fmt.Errorf("%w: expected '%s', got '%s'", ErrAuthInvalidVerbTag, verb, tTag)
	}

	if hash.Hex() != "" {
		// empty hash means don't check the 'x' tags
		xTags := allTags(event, "x")
		if len(xTags) == 0 {
			return ErrAuthMissingXTag
		}
		if !slices.Contains(xTags, hash.Hex()) {
			return fmt.Errorf("%w: missing %s", ErrAuthInvalidXTag, hash)
		}
	}

	if err := verify(event); err != nil {
		return err
	}
	return nil
}

// parseAuth parses the authentication nostr event from the provided request header.
func parseAuth(header http.Header) (*nostr.Event, error) {
	auth := header.Get("Authorization")
	if auth == "" {
		return nil, ErrAuthMissingHeader
	}

	parts := strings.Split(auth, " ")
	if len(parts) != 2 {
		return nil, ErrAuthInvalidScheme
	}
	if parts[0] != "Nostr" {
		return nil, ErrAuthInvalidScheme
	}

	bytes, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAuthInvalidBase64, err)
	}

	event := &nostr.Event{}
	if err := json.Unmarshal(bytes, event); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidEventJSON, err)
	}
	return event, nil
}

// firstTag returns the first value of the tag with the provided key, and whether it was present.
func firstTag(e *nostr.Event, key string) (string, bool) {
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == key {
			return tag[1], true
		}
	}
	return "", false
}

// allTags returns the list of values of tags with the provided key.
func allTags(e *nostr.Event, key string) []string {
	var vals []string
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == key {
			vals = append(vals, tag[1])
		}
	}
	return vals
}

// Verify returns an error if the event has invalid ID or signature, nil otherwise.
func verify(e *nostr.Event) error {
	if !e.CheckID() {
		return ErrInvalidEventID
	}

	match, err := e.CheckSignature()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidEventSig, err)
	}
	if !match {
		return ErrInvalidEventSig
	}
	return nil
}
