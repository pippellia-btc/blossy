package blossy

import (
	"context"
	"encoding/json"
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
	case r.URL.Path == "/upload" && r.Method == http.MethodPut:
		s.HandleUpload(w, r)

	case r.URL.Path == "/upload" && r.Method == http.MethodHead:
		s.HandleUploadCheck(w, r)

	case r.Method == http.MethodGet:
		s.HandleFetchBlob(w, r)

	case r.Method == http.MethodHead:
		s.HandleFetchMeta(w, r)

	case r.Method == http.MethodOptions:
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Unsupported request", http.StatusBadRequest)
	}
}

// HandleFetchBlob handles the GET /<sha256>.<ext> endpoint.
func (s *Server) HandleFetchBlob(w http.ResponseWriter, r *http.Request) {
	request, err := parseFetch(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	for _, reject := range s.Reject.FetchBlob {
		err = reject(request, request.hash, request.ext)
		if err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	data, err := s.On.FetchBlob(request, request.hash, request.ext)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	blob := blossom.Blob{Data: data}
	if err := blossom.WriteBlob(w, blob); err != nil {
		s.log.Error("failure in GET /<sha256>", "error", err)
	}
}

// HandleFetchMeta handles the HEAD /<sha256>.<ext> endpoint.
func (s *Server) HandleFetchMeta(w http.ResponseWriter, r *http.Request) {
	request, err := parseFetch(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	for _, reject := range s.Reject.FetchMeta {
		err = reject(request, request.hash, request.ext)
		if err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	mime, size, err := s.On.FetchMeta(request, request.hash, request.ext)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Accept-Ranges", "bytes")
}

// HandleUpload handles the PUT /upload endpoint.
func (s *Server) HandleUpload(w http.ResponseWriter, r *http.Request) {
	request, err := parseUpload(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	for _, reject := range s.Reject.Upload {
		err = reject(request, request.hints)
		if err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	meta, err := s.On.Upload(request, request.hints, request.body)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	descriptor := BlobDescriptor{
		URL:      s.baseURL + "/" + meta.Hash.Hex() + meta.Extension(),
		SHA256:   meta.Hash.Hex(),
		Size:     meta.Size,
		Type:     meta.Type,
		Uploaded: meta.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(descriptor); err != nil {
		s.log.Error("failed to encode blob descriptor", "error", err, "hash", meta.Hash)
	}
}

// HandleUploadCheck handles the HEAD /upload endpoint.
func (s *Server) HandleUploadCheck(w http.ResponseWriter, r *http.Request) {
	request, err := parseUploadCheck(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	for _, reject := range s.Reject.Upload {
		err = reject(request, request.hints)
		if err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	request, err := parseDelete(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	for _, reject := range s.Reject.Delete {
		err = reject(request, request.hash)
		if err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	err = s.On.Delete(request, request.hash)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// SetCORS sets CORS headers as required by BUD-01.
func SetCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, *")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
}
