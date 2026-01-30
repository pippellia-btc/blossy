package blossy

import (
	"errors"
	"log/slog"
	"net/url"
	"time"
)

type Option func(*Server)

// WithBaseURL sets the server base URL, which is be used in [BlobDescriptor],
// and will be used in validating auth (not yet implemented).
// If not set, a warning will be logged.
func WithBaseURL(url string) Option {
	return func(s *Server) {
		s.Sys.baseURL = url
	}
}

// WithLogger sets the structured logger (*slog.Logger) used by the relay for all logging operations.
// If not set, a default logger will be used.
func WithLogger(l *slog.Logger) Option {
	return func(s *Server) {
		s.log = l
	}
}

// WithReadHeaderTimeout sets the maximum duration for reading the headers of an HTTP request.
// It's used only in the http server used by [Server.StartAndServe]. Must be > 1s.
func WithReadHeaderTimeout(d time.Duration) Option {
	return func(s *Server) { s.settings.HTTP.readHeaderTimeout = d }
}

// WithIdleTimeout sets the maximum duration an HTTP connection can be idle before being closed.
// It's used only in the http server used by [Server.StartAndServe]. Must be > 10s.
func WithIdleTimeout(d time.Duration) Option {
	return func(s *Server) { s.settings.HTTP.idleTimeout = d }
}

// WithShutdownTimeout sets the maximum duration to wait for the HTTP server to gracefully shut down
// when the context is cancelled. It's used only in the http server used by [Server.StartAndServe].
func WithShutdownTimeout(d time.Duration) Option {
	return func(s *Server) { s.settings.HTTP.shutdownTimeout = d }
}

// settings holds the configurable parameters for the Relay.
type settings struct {
	Sys  systemSettings
	HTTP httpSettings
}

func newSettings() settings {
	return settings{
		HTTP: newHTTPSettings(),
	}
}

type systemSettings struct {
	// BaseURL is the server base URL, used in the returned [BlobDescriptor].
	// It will also be used in validating auth (not yet implemented, see auth.go).
	baseURL string
}

// httpSettings holds the configurable parameters for the default HTTP server, which is
// used when calling [Server.StartAndServe].
type httpSettings struct {
	readHeaderTimeout time.Duration
	idleTimeout       time.Duration
	shutdownTimeout   time.Duration
}

func newHTTPSettings() httpSettings {
	return httpSettings{
		readHeaderTimeout: 5 * time.Second,
		idleTimeout:       120 * time.Second,
		shutdownTimeout:   5 * time.Second,
	}
}

func (s *Server) validate() error {
	// sys
	if s.settings.Sys.baseURL == "" {
		s.log.Warn("server base url is not set. This means you will have to manually set the URL of all blob descriptors returned")
	} else {
		if _, err := url.Parse(s.settings.Sys.baseURL); err != nil {
			return errors.New("invalid server base url: " + err.Error())
		}
	}

	// http
	if s.settings.HTTP.readHeaderTimeout < 1*time.Second {
		return errors.New("http read header timeout must be greater than 1s to function reliably")
	}
	if s.settings.HTTP.idleTimeout < 10*time.Second {
		return errors.New("http idle timeout must be greater than 10s to function reliably")
	}
	if s.settings.HTTP.shutdownTimeout < 1*time.Second {
		return errors.New("http shutdown timeout should be greater than 1s to avoid abrupt disconnections")
	}
	return nil
}
