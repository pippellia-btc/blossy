package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/pippellia-btc/blossom"
	"github.com/pippellia-btc/blossy"
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

	blossom.On.Download = BlobNotFound
	blossom.On.Check = MetaNotFound

	err = blossom.StartAndServe(ctx, "localhost:3335")
	if err != nil {
		panic(err)
	}
}

func BlobNotFound(r blossy.Request, hash blossom.Hash, ext string) (blossy.BlobDelivery, *blossom.Error) {
	slog.Info("received GET request", "hash", hash, "ext", ext, "ip", r.IP().Group())
	return nil, blossom.ErrNotFound("Blob not found")
}

func MetaNotFound(r blossy.Request, hash blossom.Hash, ext string) (blossy.MetaDelivery, *blossom.Error) {
	slog.Info("received HEAD request", "hash", hash, "ext", ext, "ip", r.IP().Group())
	return nil, blossom.ErrNotFound("Blob not found")
}
