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
		blossy.WithHostname("example.com"),
	)
	if err != nil {
		panic(err)
	}

	blossom.On.Download = LoadBlob
	blossom.On.Check = LoadMeta
	blossom.On.Upload = SaveBlob
	blossom.On.Delete = DeleteBlob

	err = blossom.StartAndServe(ctx, "localhost:3335")
	if err != nil {
		panic(err)
	}
}

func LoadBlob(r blossy.Request, hash blossom.Hash, ext string) (blossy.BlobDelivery, *blossom.Error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	file, err := store.Load(ctx, hash)
	if errors.Is(err, blisk.ErrNotFound) {
		return nil, blossom.ErrNotFound("Blob not found")
	}
	if err != nil {
		return nil, blossom.ErrInternal(err.Error())
	}

	blob, err := blossom.BlobFromFile(file)
	if err != nil {
		return nil, blossom.ErrInternal(err.Error())
	}
	return blossy.Serve(blob), nil
}

func LoadMeta(r blossy.Request, hash blossom.Hash, ext string) (blossy.MetaDelivery, *blossom.Error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	meta, err := store.Info(ctx, hash)
	if errors.Is(err, blisk.ErrNotFound) {
		return nil, blossom.ErrNotFound("Blob not found")
	}
	if err != nil {
		return nil, blossom.ErrInternal(err.Error())
	}

	return blossy.Found(meta.Type, meta.Size), nil
}

func SaveBlob(r blossy.Request, hints blossy.UploadHints, data io.Reader) (blossom.BlobDescriptor, *blossom.Error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	meta, err := store.Save(ctx, data, r.Pubkey())
	if err != nil {
		return blossom.BlobDescriptor{}, blossom.ErrInternal(err.Error())
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
		return blossom.ErrNotFound("Blob not found")
	}
	if err != nil {
		return blossom.ErrInternal(err.Error())
	}
	return nil
}
