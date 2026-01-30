package blossy

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
	"sync/atomic"

	"github.com/pippellia-btc/blossom"
)

// Server is the fundamental structure of the blossy package.
// Create one with [NewServer].
type Server struct {
	log         *slog.Logger
	nextRequest atomic.Int64

	Hooks
	settings
}

// NewServer creates a new Server instance with sane defaults and customizable internal behavior.
// Customize its structure with functional options (e.g., [WithBaseURL], [WithReadHeaderTimeout]).
// Customize its behaviour by defining On.FetchBlob, On.Upload and other [Hooks].
//
// Example:
//
//	blossom := NewServer(
//	    WithBaseURL("example.com"),
//	    WithReadHeaderTimeout(5 * time.Second),
//	)
func NewServer(opts ...Option) (*Server, error) {
	server := &Server{
		log:      slog.Default(),
		Hooks:    DefaultHooks(),
		settings: newSettings(),
	}

	for _, opt := range opts {
		opt(server)
	}

	if err := server.validate(); err != nil {
		return nil, err
	}
	return server, nil
}

// TotalRequests returns the total number of requests received since the server startup.
func (s *Server) TotalRequests() int {
	return int(s.nextRequest.Load())
}

// deriveURL derives the URL for a blob descriptor.
// If the server base URL is not set, it returns an error.
func (s *Server) deriveURL(desc blossom.BlobDescriptor) (string, error) {
	if s.Sys.baseURL == "" {
		return "", errors.New("server base url is not set")
	}
	return s.Sys.baseURL + "/" + desc.Hash.Hex() + blossom.ExtFromType(desc.Type), nil
}

// StartAndServe starts the blossom server, listens to the provided address and handles http requests.
//
// It's a blocking operation, that stops only when the context gets cancelled.
func (s *Server) StartAndServe(ctx context.Context, address string) error {
	exitErr := make(chan error, 1)
	server := &http.Server{
		Addr:              address,
		Handler:           s,
		ReadHeaderTimeout: s.settings.HTTP.readHeaderTimeout,
		IdleTimeout:       s.settings.HTTP.idleTimeout,
	}

	go func() {
		s.log.Info("serving the blossom server", "address", address)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			exitErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), s.settings.HTTP.shutdownTimeout)
		defer cancel()

		s.log.Info("shutting down the blossom server", "address", address)
		return server.Shutdown(ctx)

	case err := <-exitErr:
		return err
	}
}

// ServeHTTP implements the [http.Handler] interface, routing http requests to the appropriate [Hook].
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	switch {
	case r.URL.Path == "/upload" && r.Method == http.MethodPut:
		s.HandleUpload(w, r)

	case r.URL.Path == "/upload" && r.Method == http.MethodHead:
		s.HandleUploadCheck(w, r)

	case r.URL.Path == "/media" && r.Method == http.MethodPut:
		s.HandleMedia(w, r)

	case r.URL.Path == "/media" && r.Method == http.MethodHead:
		s.HandleMediaCheck(w, r)

	case r.URL.Path == "/mirror" && r.Method == http.MethodPut:
		s.HandleMirror(w, r)

	case r.URL.Path == "/report" && r.Method == http.MethodPut:
		s.HandleReport(w, r)

	case r.Method == http.MethodGet:
		s.HandleFetchBlob(w, r)

	case r.Method == http.MethodHead:
		s.HandleFetchMeta(w, r)

	case r.Method == http.MethodDelete:
		s.HandleDelete(w, r)

	case r.Method == http.MethodOptions:
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Unsupported request", http.StatusMethodNotAllowed)
	}
}

// HandleFetchBlob handles the GET /<sha256>.<ext> endpoint.
func (s *Server) HandleFetchBlob(w http.ResponseWriter, r *http.Request) {
	request, err := parseFetch(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}
	request.id = s.nextRequest.Add(1)

	for _, reject := range s.Reject.FetchBlob {
		if err = reject(request, request.hash, request.ext); err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	delivery, err := s.On.FetchBlob(request, request.hash, request.ext)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	switch d := delivery.(type) {
	case servedBlob:
		blob := d.Blob
		if blob == nil {
			s.log.Error("handle fetch blob: blob is nil")
			blossom.WriteError(w, blossom.Error{Code: http.StatusNotFound, Reason: "Blob not found"})
			return
		}
		defer blob.Close()

		if err := blossom.WriteBlob(w, blob); err != nil {
			s.log.Error("failure in GET /<sha256>", "error", err)
		}

	case redirectedBlob:
		http.Redirect(w, r, d.url, d.code)

	default:
		s.log.Error("handle fetch blob: unknown blob delivery type", "type", reflect.TypeOf(delivery))
		blossom.WriteError(w, blossom.Error{Code: http.StatusInternalServerError, Reason: "Unknown blob delivery type"})
		return
	}
}

// HandleFetchMeta handles the HEAD /<sha256>.<ext> endpoint.
func (s *Server) HandleFetchMeta(w http.ResponseWriter, r *http.Request) {
	request, err := parseFetch(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}
	request.id = s.nextRequest.Add(1)

	for _, reject := range s.Reject.FetchMeta {
		if err = reject(request, request.hash, request.ext); err != nil {
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

// HandleDelete handles the DELETE /<sha256> endpoint.
func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if s.On.Delete == nil {
		// delete endpoint is optional
		err := blossom.Error{Code: http.StatusNotImplemented, Reason: "The Delete hook is not configured"}
		blossom.WriteError(w, err)
		return
	}

	request, err := parseDelete(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}
	request.id = s.nextRequest.Add(1)

	for _, reject := range s.Reject.Delete {
		if err = reject(request, request.hash); err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	if err = s.On.Delete(request, request.hash); err != nil {
		blossom.WriteError(w, *err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleUpload handles the PUT /upload endpoint.
func (s *Server) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if s.On.Upload == nil {
		// upload endpoint is optional
		err := blossom.Error{Code: http.StatusNotImplemented, Reason: "The Upload hook is not configured"}
		blossom.WriteError(w, err)
		return
	}

	request, err := parseUpload(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	request.id = s.nextRequest.Add(1)
	defer request.body.Close()

	for _, reject := range s.Reject.Upload {
		if err = reject(request, request.hints); err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	desc, err := s.On.Upload(request, request.hints, request.body)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	if desc.URL == "" {
		// derive the URL if not set
		url, err := s.deriveURL(desc)
		if err != nil {
			s.log.Error("handle upload: failed to derive URL", "error", err)
			blossom.WriteError(w, blossom.Error{Code: http.StatusInternalServerError, Reason: err.Error()})
			return
		}
		desc.URL = url
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(desc); err != nil {
		s.log.Error("failed to encode blob descriptor", "error", err, "hash", desc.Hash)
	}
}

// HandleUploadCheck handles the HEAD /upload endpoint.
func (s *Server) HandleUploadCheck(w http.ResponseWriter, r *http.Request) {
	if s.On.Upload == nil {
		// upload endpoint is optional
		err := blossom.Error{Code: http.StatusNotImplemented, Reason: "The Upload hook is not configured"}
		blossom.WriteError(w, err)
		return
	}

	request, err := parseUploadCheck(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}
	request.id = s.nextRequest.Add(1)

	for _, reject := range s.Reject.Upload {
		if err = reject(request, request.hints); err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// HandleMirror handles the PUT /mirror endpoint.
func (s *Server) HandleMirror(w http.ResponseWriter, r *http.Request) {
	if s.On.Mirror == nil {
		// mirror endpoint is optional
		err := blossom.Error{Code: http.StatusNotImplemented, Reason: "The Mirror hook is not configured"}
		blossom.WriteError(w, err)
		return
	}

	request, err := parseMirror(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	request.id = s.nextRequest.Add(1)

	for _, reject := range s.Reject.Mirror {
		if err = reject(request, request.url); err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	desc, err := s.On.Mirror(request, request.url)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	if desc.URL == "" {
		// derive the URL if not set
		url, err := s.deriveURL(desc)
		if err != nil {
			s.log.Error("handle mirror: failed to derive URL", "error", err)
			blossom.WriteError(w, blossom.Error{Code: http.StatusInternalServerError, Reason: err.Error()})
			return
		}
		desc.URL = url
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(desc); err != nil {
		s.log.Error("failed to encode blob descriptor", "error", err, "hash", desc.Hash)
	}
}

// HandleMedia handles the PUT /media endpoint.
func (s *Server) HandleMedia(w http.ResponseWriter, r *http.Request) {
	if s.On.Media == nil {
		// media endpoint is optional
		err := blossom.Error{Code: http.StatusNotImplemented, Reason: "The Media hook is not configured"}
		blossom.WriteError(w, err)
		return
	}

	request, err := parseUpload(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	request.id = s.nextRequest.Add(1)
	defer request.body.Close()

	for _, reject := range s.Reject.Media {
		if err = reject(request, request.hints); err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	desc, err := s.On.Media(request, request.hints, request.body)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}

	if desc.URL == "" {
		// derive the URL if not set
		url, err := s.deriveURL(desc)
		if err != nil {
			s.log.Error("handle media: failed to derive URL", "error", err)
			blossom.WriteError(w, blossom.Error{Code: http.StatusInternalServerError, Reason: err.Error()})
			return
		}
		desc.URL = url
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(desc); err != nil {
		s.log.Error("failed to encode blob descriptor", "error", err, "hash", desc.Hash)
	}
}

// HandleMediaCheck handles the HEAD /media endpoint.
func (s *Server) HandleMediaCheck(w http.ResponseWriter, r *http.Request) {
	if s.On.Media == nil {
		// media endpoint is optional
		err := blossom.Error{Code: http.StatusNotImplemented, Reason: "The Media hook is not configured"}
		blossom.WriteError(w, err)
		return
	}

	request, err := parseUploadCheck(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}
	request.id = s.nextRequest.Add(1)

	for _, reject := range s.Reject.Media {
		if err = reject(request, request.hints); err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// HandleReport handles the PUT /report endpoint.
func (s *Server) HandleReport(w http.ResponseWriter, r *http.Request) {
	if s.On.Report == nil {
		// report endpoint is optional
		err := blossom.Error{Code: http.StatusNotImplemented, Reason: "The Report hook is not configured"}
		blossom.WriteError(w, err)
		return
	}

	request, err := parseReport(r)
	if err != nil {
		blossom.WriteError(w, *err)
		return
	}
	request.id = s.nextRequest.Add(1)

	for _, reject := range s.Reject.Report {
		if err = reject(request, request.report); err != nil {
			blossom.WriteError(w, *err)
			return
		}
	}

	if err = s.On.Report(request, request.report); err != nil {
		blossom.WriteError(w, *err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// setCORS sets CORS headers as required by BUD-01.
func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, *")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
}
