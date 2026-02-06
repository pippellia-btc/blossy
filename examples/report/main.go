package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"slices"

	"github.com/pippellia-btc/blossom"
	"github.com/pippellia-btc/blossy"
)

/*
This example shows how to deal with BUD-09 reports.
A report from one of the moderators will delete all the blobs it reference.
A report from a non-moderator will be saved for the operator to review and take action manually.
*/

// a slice of pubkeys that act as moderators for the blossom server.
var moderators []string

// a slice of reports to be reviews manually by the server operator.
// In production you would use a database instead.
var toReview []blossy.Report

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	blossom, err := blossy.NewServer(
		blossy.WithHostname("example.com"),
	)
	if err != nil {
		panic(err)
	}

	blossom.On.Report = DeleteOrNotify

	err = blossom.StartAndServe(ctx, "localhost:3335")
	if err != nil {
		panic(err)
	}
}

func DeleteOrNotify(r blossy.Request, report blossy.Report) *blossom.Error {
	if slices.Contains(moderators, report.Pubkey) {
		hashes := report.Hashes()
		slog.Info("deleting blobs", "hashes", hashes)
	}

	toReview = append(toReview, report)
	slog.Info("new report to review", "report", report)
	return nil
}
