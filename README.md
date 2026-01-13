# Blossy

A framework for building super custom [Blossom](https://github.com/hzrd149/blossom) servers. Written in Go, it's designed to be simple and performant, while providing an exeptional developer experience.

<a href="https://pkg.go.dev/github.com/pippellia-btc/blossy"><img src="https://pkg.go.dev/badge/github.com/pippellia-btc/blossy.svg" alt="Go Reference"></a>
[![Go Report Card](https://goreportcard.com/badge/github.com/pippellia-btc/blossy)](https://goreportcard.com/report/github.com/pippellia-btc/blossy)

## Installation
```
go get github.com/pippellia-btc/blossy
```

## Simple and Customizable
Getting started is easy, and deep customization is just as straightforward.

```golang
package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/pippellia-btc/rely"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	blossom, err := blossy.NewServer(
		blossy.WithBaseURL("example.com"),
	)
	if err != nil {
		panic(err)
	}

	err = blossom.StartAndServe(ctx, "localhost:3335")
	if err != nil {
		panic(err)
	}
}
```

### Structural Customization
Fine-tune core parameters using functional options:

```golang
blossom := blossy.NewServer(
	blossy.WithBaseURL("myDomain.com"),	    // required for blob descriptors and auth
	blossy.WithLogger(myLogger),		    // configure the server logger
	blossy.WithReadHeaderTimeout(timeout)	// choose http security settings
)
```

To find all the available options and documentation, see [options.go](/options.go).

### Behavioral Customization

You are not limited to simple configuration variables. The server architecture facilitates complete behavioral customization by allowing you to inject your own functions into its `Hooks`. This gives you full control over the connection lifecycle, blob flow and rate-limiting, enabling any custom business logic.

Below is a silly example that illustrates blossy's flexibility.

```golang
func main() {
	// ...
	blossom.Reject.FetchBlob.Append(IsWord)
	blossom.Reject.Upload.Append(BadIP)
    blossom.On.Upload = Save // your custom DB save
}

func BadIP(r blossy.Request, hints blossy.UploadHints) *blossom.Error {
	if slices.Contains(blacklist, r.IP().Group()) {
		return &blossom.Error{Code: http.StatusForbidden, Reason: "you shall not pass!"}
	}
	return nil
}

func IsWord(r blossy.Request, hash blossom.Hash, ext string) *blossom.Error {
	if ext == "docx" || ext == "doc" {
		blacklist = append(blacklist, r.IP().Group())
		return &blossom.Error{Code: http.StatusUnsupportedMediaType, Reason: "We don't like Microsoft"}
	}
	return nil
}
```

## Databases

Blossy doesn't come with a default database, you have to provide your own.  
Fortunately, the community has developed several ready-to-use database implementations:  
- [blisk](https://github.com/pippellia-btc/blisk): a local database for storing blossom blobs on disk. It is designed for efficient, scalable, and deduplicated blob storage while maintaining metadata in sqlite. It's short for Blobs on Disk.

## Security

The authorization spec used by the Blossom protocol at the time of writing is not secure against replay attacks. Therefore, the implementation that blossy uses, by adhering to the protocol, is to be considered not secure.

Hopefully, the protocol will adopt more secure specs like [Nostr Web Tokens](https://github.com/pippellia-btc/nostr-web-tokens).