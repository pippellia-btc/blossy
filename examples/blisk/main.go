package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/pippellia-btc/blisk"
	"github.com/pippellia-btc/blossom"
	"github.com/pippellia-btc/blossy"
)

var store *blisk.Store

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	var err error
	store, err = blisk.New(".blossom")
	if err != nil {
		panic(err)
	}
	defer store.Close()

	blossom, err := blossy.NewServer(
		blossy.WithBaseURL("example.com"),
	)
	if err != nil {
		panic(err)
	}

	blossom.On.FetchBlob = LoadBlob
	blossom.On.FetchMeta = LoadMeta
	blossom.On.Upload = SaveBlob
	blossom.On.Delete = DeleteBlob

	err = blossom.StartAndServe(ctx, "localhost:3335")
	if err != nil {
		panic(err)
	}
}

func LoadBlob(r blossy.Request, hash blossom.Hash, ext string) (blossom.Blob, *blossom.Error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	file, err := store.Load(ctx, hash)
	if errors.Is(err, blisk.ErrNotFound) {
		return nil, &blossom.Error{Code: 404, Reason: "Blob not found"}
	}
	if err != nil {
		return nil, &blossom.Error{Code: 500, Reason: err.Error()}
	}

	blob, err := blossom.BlobFromFile(file)
	if err != nil {
		return nil, &blossom.Error{Code: 500, Reason: err.Error()}
	}
	return blob, nil
}

func LoadMeta(r blossy.Request, hash blossom.Hash, ext string) (string, int64, *blossom.Error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	meta, err := store.Info(ctx, hash)
	if errors.Is(err, blisk.ErrNotFound) {
		return "", 0, &blossom.Error{Code: 404, Reason: "Blob not found"}
	}
	if err != nil {
		return "", 0, &blossom.Error{Code: 500, Reason: err.Error()}
	}

	return meta.Type, meta.Size, nil
}

func SaveBlob(r blossy.Request, hints blossy.UploadHints, data io.Reader) (blossom.BlobDescriptor, *blossom.Error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	meta, err := store.Save(ctx, data, r.Pubkey())
	if err != nil {
		return blossom.BlobDescriptor{}, &blossom.Error{Code: 500, Reason: err.Error()}
	}

	return blossom.BlobDescriptor{
		Hash:     meta.Hash,
		Size:     meta.Size,
		Type:     meta.Type,
		Uploaded: meta.CreatedAt,
	}, nil
}

func DeleteBlob(r blossy.Request, hash blossom.Hash) *blossom.Error {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := store.Delete(ctx, hash, r.Pubkey())
	if errors.Is(err, blisk.ErrNotFound) {
		return &blossom.Error{Code: 404, Reason: "Blob not found"}
	}
	if err != nil {
		return &blossom.Error{Code: 500, Reason: err.Error()}
	}
	return nil
}
