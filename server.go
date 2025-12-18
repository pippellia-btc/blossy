package blossy

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pippellia-btc/blossom"
)

type Server struct {
	baseURL string
	nextID  atomic.Int64
	log     *slog.Logger
	Hooks
}

type Option func(*Server)

func WithBaseURL(url string) Option {
	return func(s *Server) {
		s.baseURL = url
	}
}

func WithLogger(l *slog.Logger) Option {
	return func(s *Server) {
		s.log = l
	}
}

// NewServer returns a blossom server initialized with default parameters.
func NewServer(opts ...Option) (*Server, error) {
	server := &Server{
		log:   slog.Default(),
		Hooks: DefaultHooks(),
	}

	for _, opt := range opts {
		opt(server)
	}

	if err := server.validate(); err != nil {
		return nil, err
	}
	return server, nil
}

func (s *Server) validate() error {
	return nil
}

// StartAndServe starts the blossom server, listens to the provided address and handles http requests.
//
// It's a blocking operation, that stops only when the context gets cancelled.
func (s *Server) StartAndServe(ctx context.Context, address string) error {
	exitErr := make(chan error, 1)
	server := &http.Server{Addr: address, Handler: s}

	go func() {
		s.log.Info("serving the blossom server", "address", address)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			exitErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(ctx)

	case err := <-exitErr:
		return err
	}
}

// ServeHTTP implements the [http.Handler] interface, routing http requests to the appropriate [Hook].
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	SetCORS(w)

	switch {
	case r.Method == http.MethodGet:
		s.HandleGet(w, r)

	case r.Method == http.MethodHead:
		s.HandleCheck(w, r)

	case r.Method == http.MethodOptions:
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Unsupported request", http.StatusBadRequest)
	}
}

// HandleGet handles the GET /<sha256>.<ext> endpoint.
func (s *Server) HandleGet(w http.ResponseWriter, r *http.Request) {
	request, err := parseBlobRequest(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	for _, reject := range s.Reject.Get {
		err = reject(request, request.hash, request.ext)
		if err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	data, err := s.On.Get(request, request.hash, request.ext)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	blob := blossom.Blob{Data: data}
	if err := blossom.WriteBlob(w, blob); err != nil {
		s.log.Error("failure in GET /<sha256>", "error", err)
	}
}

// HandleCheck handles the HEAD /<sha256>.<ext> endpoint.
func (s *Server) HandleCheck(w http.ResponseWriter, r *http.Request) {
	request, err := parseBlobRequest(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	for _, reject := range s.Reject.Check {
		err = reject(request, request.hash, request.ext)
		if err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	mime, size, err := s.On.Check(request, request.hash, request.ext)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Accept-Ranges", "bytes")
}
