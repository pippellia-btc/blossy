package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"

	"github.com/pippellia-btc/blossom"
	"github.com/pippellia-btc/blossy"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	blossom, err := blossy.NewServer()
	if err != nil {
		panic(err)
	}

	blossom.On.FetchBlob = BlobNotFound
	blossom.On.FetchMeta = MetaNotFound

	err = blossom.StartAndServe(ctx, "localhost:3335")
	if err != nil {
		panic(err)
	}
}

func BlobNotFound(r blossy.Request, hash blossom.Hash, ext string) (io.ReadSeekCloser, *blossom.Error) {
	slog.Info("received GET request", "hash", hash, "ext", ext, "ip", r.IP().Group())
	return nil, &blossom.Error{Code: 404, Reason: "Blob not found"}
}

func MetaNotFound(r blossy.Request, hash blossom.Hash, ext string) (string, int64, *blossom.Error) {
	slog.Info("received HEAD request", "hash", hash, "ext", ext, "ip", r.IP().Group())
	return "", 0, &blossom.Error{Code: 404, Reason: "Blob not found"}
}
