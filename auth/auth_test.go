package auth

import (
	"fmt"
	"net/http"
	"testing"
)

func TestImpliedAction(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		want    Action
		isValid bool
	}{
		// upload paths
		{"PUT /upload", http.MethodPut, "/upload", ActionUpload, true},
		{"HEAD /upload", http.MethodHead, "/upload", ActionUpload, true},
		{"PUT /media", http.MethodPut, "/media", ActionUpload, true},
		{"HEAD /media", http.MethodHead, "/media", ActionUpload, true},
		{"PUT /mirror", http.MethodPut, "/mirror", ActionUpload, true},

		// list
		{"GET /list/pubkey", http.MethodGet, "/list/abc123", ActionList, true},
		{"GET /list", http.MethodGet, "/list", ActionList, true},

		// get
		{"GET /hash.ext", http.MethodGet, "/abcdef.png", ActionGet, true},
		{"HEAD /hash.ext", http.MethodHead, "/abcdef.png", ActionGet, true},

		// delete
		{"DELETE /hash", http.MethodDelete, "/abcdef", ActionDelete, true},

		// no false positive on "list" substring
		{"GET /playlist", http.MethodGet, "/playlist", ActionGet, true},

		// path without leading slash
		{"PUT upload", http.MethodPut, "upload", ActionUpload, true},

		// invalid
		{"POST /unknown", http.MethodPost, "/unknown", "", false},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d_%s", i, test.name), func(t *testing.T) {
			r, _ := http.NewRequest(test.method, test.path, nil)
			got, err := impliedAction(r)

			if !test.isValid {
				if err == nil {
					t.Fatalf("expected error for %s %s, got nil", test.method, test.path)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for %s %s: %v", test.method, test.path, err)
			}
			if got != test.want {
				t.Errorf("expected %q, got %q", test.want, got)
			}
		})
	}
}
