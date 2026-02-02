package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"sync"

	"github.com/pippellia-btc/blossom"
	"github.com/pippellia-btc/blossy"
)

// the list of banned IPs, protected by mu
var (
	blacklist []string
	mu        sync.RWMutex
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	blossom, err := blossy.NewServer(
		blossy.WithBaseURL("https://example.com"),
	)
	if err != nil {
		panic(err)
	}

	blossom.Reject.Download.Append(IsWord)
	blossom.Reject.Upload.Append(BadIP)

	err = blossom.StartAndServe(ctx, "localhost:3335")
	if err != nil {
		panic(err)
	}
}

func BadIP(r blossy.Request, hints blossy.UploadHints) *blossom.Error {
	mu.RLock()
	defer mu.RUnlock()

	if slices.Contains(blacklist, r.IP().Group()) {
		return blossom.ErrForbidden("you shall not pass!")
	}
	return nil
}

func IsWord(r blossy.Request, hash blossom.Hash, ext string) *blossom.Error {
	if ext == "docx" || ext == "doc" {
		ip := r.IP().Group()
		slog.Info("blacklisting", "IP", ip)

		mu.Lock()
		defer mu.Unlock()

		blacklist = append(blacklist, ip)
		return blossom.ErrUnsupportedMedia("We don't like Microsoft")
	}
	return nil
}
