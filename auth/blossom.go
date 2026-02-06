package auth

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/blossom"
)

const (
	KindBlossomAuth  = 24242
	DefaultClockSkew = 10 * time.Second
	MaxTags          = 512
)

// BlossomAuth represents a parsed Blossom authorization event.
type BlossomAuth struct {
	Pubkey     string
	CreatedAt  time.Time
	Expiration time.Time
	Action     Action
	Hashes     []blossom.Hash
	Hostnames  []string
}

// Validate validates the Blossom authorization event time bounds and
// against the expected action, hash and server hostname.
func (a *BlossomAuth) Validate(action Action, hash blossom.Hash, hostname string) error {
	now := time.Now()
	min := now.Add(-DefaultClockSkew)
	max := now.Add(DefaultClockSkew)
	if a.CreatedAt.After(max) {
		return errors.New("event created at is in the future")
	}
	if a.Expiration.Before(min) {
		return errors.New("event expiration is in the past")
	}

	if a.Action != action {
		return fmt.Errorf("expected action %s, got %s", action, a.Action)
	}

	if len(a.Hashes) > 0 {
		// no x tags means the event is considered valid for all blobs.
		if !slices.Contains(a.Hashes, hash) {
			return fmt.Errorf("expected hash %s, got %s", hash, a.Hashes)
		}
	}

	if len(a.Hostnames) > 0 {
		// no server tags means the event is considered valid for all servers.
		if !slices.Contains(a.Hostnames, hostname) {
			return fmt.Errorf("expected server hostname %s, got %s", hostname, a.Hostnames)
		}
	}
	return nil
}

// ParseBlossomAuth parses the Blossom authentication event from the provided Nostr event.
// It returns an error if the event is structurally invalid, but doesn't validate the event
// against the expected claims.
func ParseBlossomAuth(e *nostr.Event) (*BlossomAuth, error) {
	if e == nil {
		return nil, errors.New("event is nil")
	}
	if e.Kind != KindBlossomAuth {
		return nil, errors.New("event kind is not 24242")
	}
	if len(e.Tags) > MaxTags {
		return nil, errors.New("event has too many tags")
	}

	auth := &BlossomAuth{
		Pubkey:    e.PubKey,
		CreatedAt: e.CreatedAt.Time(),
	}

	foundT := false
	foundExp := false

	for _, tag := range e.Tags {
		if len(tag) < 2 {
			continue
		}

		switch tag[0] {
		case "t":
			if foundT {
				return nil, errors.New("'t' tag appears multiple times")
			}
			foundT = true

			if !slices.Contains(validActions, Action(tag[1])) {
				return nil, fmt.Errorf("invalid 't' tag: %s", tag[1])
			}
			auth.Action = Action(tag[1])

		case "expiration":
			if foundExp {
				return nil, errors.New("'expiration' tag appears multiple times")
			}
			foundExp = true

			unix, err := strconv.ParseInt(tag[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("'expiration' tag is not a valid unix time: %w", err)
			}
			auth.Expiration = time.Unix(unix, 0).UTC()

		case "x":
			hash, err := blossom.ParseHash(tag[1])
			if err == nil {
				// only append valid hashes as the validation just needs the matching "x" tag.
				auth.Hashes = append(auth.Hashes, hash)
			}

		case "server":
			auth.Hostnames = append(auth.Hostnames, tag[1])
		}
	}

	if !foundT {
		return nil, errors.New("'t' tag is missing")
	}
	if !foundExp {
		return nil, errors.New("'expiration' tag is missing")
	}
	return auth, nil
}
