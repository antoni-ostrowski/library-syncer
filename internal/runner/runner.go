package runner

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/antoni-ostrowski/library-syncer/internal/db"
	"github.com/antoni-ostrowski/library-syncer/internal/downloader"
	srccsv "github.com/antoni-ostrowski/library-syncer/internal/gsh"
	"github.com/antoni-ostrowski/library-syncer/internal/parser"
)

type Runner struct {
	running          atomic.Bool
	manual           chan struct{}
	db               *db.DbService
	devMode          bool
	sleepDuration    time.Duration
	tracksToDownload chan downloader.Downloadable
}

func New(db *db.DbService, sleepSec int, devMode bool) *Runner {
	return &Runner{db: db, tracksToDownload: make(chan downloader.Downloadable, 10000), sleepDuration: time.Duration(sleepSec) * time.Second, devMode: devMode, manual: make(chan struct{}, 1)}
}

func (r *Runner) Start(ctx context.Context) {
	timer := time.NewTicker(r.sleepDuration)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			r.run(ctx)
			timer.Reset(r.sleepDuration)
		case <-r.manual:
			r.run(ctx)
			timer.Reset(r.sleepDuration)
		case <-ctx.Done():
			return
		}
	}
}
func (r *Runner) Trigger() {
	select {
	case r.manual <- struct{}{}:
	default:
	}

}

func (r *Runner) run(ctx context.Context) {
	if !r.running.CompareAndSwap(false, true) {
		return
	}
	defer r.running.Store(false)

	trackers, err := r.db.ListTrackers(ctx)
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}

	downloader.DownloadTracks(ctx, r.devMode, r.tracksToDownload)
	for _, v := range trackers {
		ExecuteTracker(ctx, r.db, v, r.tracksToDownload)
	}
	fmt.Printf("---sleeping---\n")

}

func ExecuteTracker(ctx context.Context, db *db.DbService, tracker parser.Tracker, tracksToDownload chan<- downloader.Downloadable) {
	fmt.Printf("running for %v\n", tracker.Artist)
	upTracker := tracker
	upTracker.Status = "syncing"
	if err := db.UpsertTracker(ctx, upTracker); err != nil {
		log.Printf("failed to mark syncing: %v", err)
		return
	}
	for _, readRange := range tracker.ReadRanges {
		csvPath, err := srccsv.DownloadSourceCsv(ctx, tracker.Id, readRange)
		if err != nil {
			fmt.Printf("failed to download source csv: %v\n", err)
			return
		}
		fmt.Printf("csv at %v\n", csvPath)

		sourceTracks, err := parser.Parse(csvPath, tracker)
		if err != nil {
			fmt.Printf("failed to parse source csv: %v\n", err)
			return
		}
		fmt.Printf("%v source tracks found\n", len(sourceTracks))

		trackerId := tracker.Id + "#" + readRange
		syncResult, err := db.SyncTracks(ctx, &sourceTracks, trackerId)
		if err != nil {
			fmt.Printf("failed to sync tracks to db: %v\n", err)
			return
		}
		fmt.Println(syncResult)
		for _, v := range syncResult.TracksToDownload {
			tracksToDownload <- v
		}
	}

	upTracker.Status = "synced"
	if err := db.UpsertTracker(ctx, upTracker); err != nil {
		log.Printf("failed to mark final status: %v", err)
	}

}
