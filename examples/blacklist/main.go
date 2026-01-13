package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"

	"github.com/pippellia-btc/blossom"
	"github.com/pippellia-btc/blossy"
)

// the list of banned IPs
var blacklist []string

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	blossom, err := blossy.NewServer(
		blossy.WithBaseURL("example.com"),
	)
	if err != nil {
		panic(err)
	}

	blossom.Reject.FetchBlob.Append(IsWord)
	blossom.Reject.Upload.Append(BadIP)

	err = blossom.StartAndServe(ctx, "localhost:3335")
	if err != nil {
		panic(err)
	}
}

func BadIP(r blossy.Request, hints blossy.UploadHints) *blossom.Error {
	if slices.Contains(blacklist, r.IP().Group()) {
		return &blossom.Error{Code: http.StatusForbidden, Reason: "you shall not pass!"}
	}
	return nil
}

func IsWord(r blossy.Request, hash blossom.Hash, ext string) *blossom.Error {
	if ext == "docx" || ext == "doc" {
		ip := r.IP().Group()
		slog.Info("blacklisting", "IP", ip)
		blacklist = append(blacklist, ip)
		return &blossom.Error{Code: http.StatusUnsupportedMediaType, Reason: "We don't like Microsoft"}
	}
	return nil
}
