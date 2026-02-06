package blossy

import (
	"errors"
	"net/url"
	"strings"
)

func validateHostname(hostname string) error {
	if hostname == "" {
		return errors.New("hostname must not be empty")
	}
	if strings.Contains(hostname, "://") {
		return errors.New("hostname must not include a scheme (e.g. use \"cdn.example.com\" instead of \"https://cdn.example.com\")")
	}

	u, err := url.Parse("https://" + hostname)
	if err != nil {
		return errors.New("invalid hostname: " + err.Error())
	}
	if u.Host != hostname {
		return errors.New("hostname must be a valid domain without path, query, or fragment")
	}
	return nil
}
