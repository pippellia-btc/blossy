package auth

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/blossom"
)

var (
	testHash, _ = blossom.ParseHash("aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd")
	testPubkey  = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	futureExp   = strconv.FormatInt(time.Now().Add(5*time.Minute).Unix(), 10)
)

func TestParseBlossomAuth(t *testing.T) {
	tests := []struct {
		name    string
		event   *nostr.Event
		isValid bool
	}{
		{
			name: "valid",
			event: &nostr.Event{
				Kind:      KindBlossomAuth,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"t", "upload"},
					{"expiration", futureExp},
					{"x", testHash.Hex()},
					{"server", "cdn.example.com"},
				},
			},
			isValid: true,
		},
		{
			name: "no x tags",
			event: &nostr.Event{
				Kind:      KindBlossomAuth,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"t", "get"},
					{"expiration", futureExp},
				},
			},
			isValid: true,
		},
		{
			name: "no server tags",
			event: &nostr.Event{
				Kind:      KindBlossomAuth,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"t", "delete"},
					{"expiration", futureExp},
					{"x", testHash.Hex()},
				},
			},
			isValid: true,
		},
		{
			name: "invalid x tag skipped",
			event: &nostr.Event{
				Kind:      KindBlossomAuth,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"t", "upload"},
					{"expiration", futureExp},
					{"x", "not-a-hash"},
				},
			},
			isValid: true,
		},
		{
			name: "short tags skipped",
			event: &nostr.Event{
				Kind:      KindBlossomAuth,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"t", "upload"},
					{"expiration", futureExp},
					{"t"},
				},
			},
			isValid: true,
		},
		{
			name:    "nil event",
			event:   nil,
			isValid: false,
		},
		{
			name: "wrong kind",
			event: &nostr.Event{
				Kind:      1,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"t", "upload"},
					{"expiration", futureExp},
				},
			},
			isValid: false,
		},
		{
			name: "missing t tag",
			event: &nostr.Event{
				Kind:      KindBlossomAuth,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"expiration", futureExp},
				},
			},
			isValid: false,
		},
		{
			name: "missing expiration",
			event: &nostr.Event{
				Kind:      KindBlossomAuth,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"t", "upload"},
				},
			},
			isValid: false,
		},
		{
			name: "duplicate t tag",
			event: &nostr.Event{
				Kind:      KindBlossomAuth,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"t", "upload"},
					{"t", "get"},
					{"expiration", futureExp},
				},
			},
			isValid: false,
		},
		{
			name: "duplicate expiration",
			event: &nostr.Event{
				Kind:      KindBlossomAuth,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"t", "upload"},
					{"expiration", futureExp},
					{"expiration", "9999999999"},
				},
			},
			isValid: false,
		},
		{
			name: "unknown action",
			event: &nostr.Event{
				Kind:      KindBlossomAuth,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"t", "fly"},
					{"expiration", futureExp},
				},
			},
			isValid: false,
		},
		{
			name: "non-numeric expiration",
			event: &nostr.Event{
				Kind:      KindBlossomAuth,
				PubKey:    testPubkey,
				CreatedAt: nostr.Timestamp(time.Now().Unix()),
				Tags: nostr.Tags{
					{"t", "upload"},
					{"expiration", "not-a-number"},
				},
			},
			isValid: false,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d_%s", i, test.name), func(t *testing.T) {
			auth, err := ParseBlossomAuth(test.event)

			if !test.isValid {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if auth == nil {
				t.Fatal("expected non-nil auth")
			}
		})
	}
}

func TestParseBlossomAuth_Fields(t *testing.T) {
	e := &nostr.Event{
		Kind:      KindBlossomAuth,
		PubKey:    testPubkey,
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Tags: nostr.Tags{
			{"t", "upload"},
			{"expiration", futureExp},
			{"x", testHash.Hex()},
			{"server", "cdn.example.com"},
		},
	}

	auth, err := ParseBlossomAuth(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if auth.Pubkey != testPubkey {
		t.Errorf("pubkey: expected %s, got %s", testPubkey, auth.Pubkey)
	}
	if auth.Action != ActionUpload {
		t.Errorf("action: expected %s, got %s", ActionUpload, auth.Action)
	}
	if len(auth.Hashes) != 1 || auth.Hashes[0] != testHash {
		t.Errorf("hashes: expected [%s], got %v", testHash, auth.Hashes)
	}
	if len(auth.Hostnames) != 1 || auth.Hostnames[0] != "cdn.example.com" {
		t.Errorf("hostnames: expected [cdn.example.com], got %v", auth.Hostnames)
	}
}

func TestBlossomAuth_Validate(t *testing.T) {
	otherHash, _ := blossom.ParseHash("1111111111111111111111111111111111111111111111111111111111111111")
	zeroHash := blossom.Hash{}

	tests := []struct {
		name     string
		auth     BlossomAuth
		action   Action
		hash     *blossom.Hash
		hostname string
		isValid  bool
	}{
		{
			name: "valid",
			auth: BlossomAuth{
				CreatedAt:  time.Now(),
				Expiration: time.Now().Add(5 * time.Minute),
				Action:     ActionUpload,
				Hashes:     []blossom.Hash{testHash},
				Hostnames:  []string{"cdn.example.com"},
			},
			action:   ActionUpload,
			hash:     &testHash,
			hostname: "cdn.example.com",
			isValid:  true,
		},
		{
			name: "no hashes",
			auth: BlossomAuth{
				CreatedAt:  time.Now(),
				Expiration: time.Now().Add(5 * time.Minute),
				Action:     ActionGet,
			},
			action:  ActionGet,
			hash:    &testHash,
			isValid: true,
		},
		{
			name: "no hostnames",
			auth: BlossomAuth{
				CreatedAt:  time.Now(),
				Expiration: time.Now().Add(5 * time.Minute),
				Action:     ActionDelete,
				Hashes:     []blossom.Hash{testHash},
			},
			action:   ActionDelete,
			hash:     &testHash,
			hostname: "cdn.example.com",
			isValid:  true,
		},
		{
			name: "created_at future",
			auth: BlossomAuth{
				CreatedAt:  time.Now().Add(1 * time.Minute),
				Expiration: time.Now().Add(5 * time.Minute),
				Action:     ActionUpload,
			},
			action:  ActionUpload,
			isValid: false,
		},
		{
			name: "expired",
			auth: BlossomAuth{
				CreatedAt:  time.Now().Add(-10 * time.Minute),
				Expiration: time.Now().Add(-5 * time.Minute),
				Action:     ActionUpload,
			},
			action:  ActionUpload,
			isValid: false,
		},
		{
			name: "wrong action",
			auth: BlossomAuth{
				CreatedAt:  time.Now(),
				Expiration: time.Now().Add(5 * time.Minute),
				Action:     ActionGet,
			},
			action:  ActionUpload,
			isValid: false,
		},
		{
			name: "wrong hash",
			auth: BlossomAuth{
				CreatedAt:  time.Now(),
				Expiration: time.Now().Add(5 * time.Minute),
				Action:     ActionUpload,
				Hashes:     []blossom.Hash{otherHash},
			},
			action:  ActionUpload,
			hash:    &testHash,
			isValid: false,
		},
		{
			name: "wrong hostname",
			auth: BlossomAuth{
				CreatedAt:  time.Now(),
				Expiration: time.Now().Add(5 * time.Minute),
				Action:     ActionUpload,
				Hostnames:  []string{"other.example.com"},
			},
			action:   ActionUpload,
			hostname: "cdn.example.com",
			isValid:  false,
		},
		{
			name: "nil hash with x tags",
			auth: BlossomAuth{
				CreatedAt:  time.Now(),
				Expiration: time.Now().Add(5 * time.Minute),
				Action:     ActionUpload,
				Hashes:     []blossom.Hash{testHash},
			},
			action:  ActionUpload,
			hash:    nil,
			isValid: false,
		},
		{
			name: "nil hash without x tags",
			auth: BlossomAuth{
				CreatedAt:  time.Now(),
				Expiration: time.Now().Add(5 * time.Minute),
				Action:     ActionUpload,
			},
			action:  ActionUpload,
			hash:    nil,
			isValid: true,
		},
		{
			name: "zero hash with matching x tag",
			auth: BlossomAuth{
				CreatedAt:  time.Now(),
				Expiration: time.Now().Add(5 * time.Minute),
				Action:     ActionGet,
				Hashes:     []blossom.Hash{zeroHash},
			},
			action:  ActionGet,
			hash:    &zeroHash,
			isValid: true,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d_%s", i, test.name), func(t *testing.T) {
			err := test.auth.Validate(test.action, test.hash, test.hostname)

			if !test.isValid {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
