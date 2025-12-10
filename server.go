package blossom

import "sync/atomic"

type Server struct {
	nextID  atomic.Int64
	baseURL string
	Hooks
}

// type Option func(*Server) error

// func WithBaseURL()

// func NewServer(opts ...Option) (*Server, error) {
// 	server := Server{}
// }
