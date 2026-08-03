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
	manual           chan Cmd
	db               *db.DbService
	devMode          bool
	sleepDuration    time.Duration
	tracksToDownload chan downloader.Downloadable
}
type CmdType int

const (
	CmdTypeRunAll = iota
	CmdTypeRunOne
)

type Cmd struct {
	Id   string
	Type CmdType
}

func New(db *db.DbService, sleepSec int, devMode bool) *Runner {
	return &Runner{db: db, tracksToDownload: make(chan downloader.Downloadable, 10000), sleepDuration: time.Duration(sleepSec) * time.Second, devMode: devMode, manual: make(chan Cmd, 1)}
}

func (r *Runner) Start(ctx context.Context) {
	downloader.StartWorkers(ctx, r.devMode, r.tracksToDownload)
	timer := time.NewTicker(r.sleepDuration)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			r.runAll(ctx)
			timer.Reset(r.sleepDuration)
		case cmd := <-r.manual:
			switch cmd.Type {
			case CmdTypeRunAll:
				r.runAll(ctx)
			case CmdTypeRunOne:
				r.runTracker(ctx, cmd.Id)
			default:
			}
			timer.Reset(r.sleepDuration)
		case <-ctx.Done():
			return
		}
	}
}
func (r *Runner) Trigger(cmd Cmd) {
	select {
	case r.manual <- cmd:
	default:
	}
}
func (r *Runner) IsRunning() bool {
	return r.running.Load()
}

func (r *Runner) runTracker(ctx context.Context, trackerId string) {
	if !r.running.CompareAndSwap(false, true) {
		return
	}
	defer r.running.Store(false)

	tracker, err := r.db.GetTracker(ctx, trackerId)
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}
	ExecuteTracker(ctx, r.db, tracker, r.tracksToDownload)
}

func (r *Runner) runAll(ctx context.Context) {
	if !r.running.CompareAndSwap(false, true) {
		return
	}
	defer r.running.Store(false)

	trackers, err := r.db.ListTrackers(ctx)
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}

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
		csvPath, err := srccsv.DownloadSourceCsv(ctx, tracker.Id, readRange.Name)
		if err != nil {
			fmt.Printf("failed to download source csv: %v\n", err)
			return
		}
		fmt.Printf("csv at %v\n", csvPath)

		sourceTracks, err := parser.Parse(csvPath, tracker.Artist, readRange.Mapping)
		if err != nil {
			fmt.Printf("failed to parse source csv: %v\n", err)
			return
		}
		fmt.Printf("%v source tracks found\n", len(sourceTracks))

		trackerUniqueDbId := tracker.Id + "#" + readRange.Name
		syncResult, err := db.SyncTracks(ctx, &sourceTracks, trackerUniqueDbId)
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
