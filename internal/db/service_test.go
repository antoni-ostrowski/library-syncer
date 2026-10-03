package db

import (
	"context"
	"testing"

	"github.com/antoni-ostrowski/library-syncer/internal/config"
	"github.com/antoni-ostrowski/library-syncer/internal/model"
)

func newTestService(t *testing.T) *DbService {
	t.Helper()
	t.Setenv("DB_PATH", t.TempDir())
	conn, err := config.OpenDb()
	if err != nil {
		t.Fatalf("OpenDb: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return NewDbService(conn)
}

func TestSyncTracksUnionIsIdempotent(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	sheetA := []model.Track{{Id: "aaa", Artist: "yeat", Name: "song a", Link: "https://api.pillows.su/api/download/aaa"}}
	sheetB := []model.Track{{Id: "bbb", Artist: "yeat", Name: "song b", Link: "https://api.pillows.su/api/download/bbb"}}
	union := append(append([]model.Track{}, sheetA...), sheetB...)

	first, err := svc.SyncTracks(ctx, &union, "tracker1")
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if first.InsertedOrUpdated != 2 || len(first.TracksToDownload) != 2 {
		t.Fatalf("first sync = %+v, want 2 inserted", first)
	}

	second, err := svc.SyncTracks(ctx, &union, "tracker1")
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if second.InsertedOrUpdated != 0 || len(second.TracksToDownload) != 0 {
		t.Fatalf("second sync = %+v, want 0 (idempotent)", second)
	}

	stored, err := svc.GetTracksForTracker(ctx, "tracker1")
	if err != nil {
		t.Fatalf("get tracks: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored = %d tracks, want 2", len(stored))
	}
}

func TestSyncTracksDeletesMissing(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	full := []model.Track{
		{Id: "aaa", Artist: "yeat", Name: "song a"},
		{Id: "bbb", Artist: "yeat", Name: "song b"},
	}
	if _, err := svc.SyncTracks(ctx, &full, "tracker1"); err != nil {
		t.Fatalf("seed sync: %v", err)
	}

	partial := []model.Track{{Id: "aaa", Artist: "yeat", Name: "song a"}}
	res, err := svc.SyncTracks(ctx, &partial, "tracker1")
	if err != nil {
		t.Fatalf("delete sync: %v", err)
	}
	if res.DeletionsCount != 1 {
		t.Fatalf("deletions = %d, want 1", res.DeletionsCount)
	}

	stored, err := svc.GetTracksForTracker(ctx, "tracker1")
	if err != nil {
		t.Fatalf("get tracks: %v", err)
	}
	if len(stored) != 1 || stored[0].Id != "aaa" {
		t.Fatalf("stored = %+v, want only aaa", stored)
	}
}

func TestSyncTracksMetadataEditRequeues(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	orig := []model.Track{{Id: "aaa", Artist: "yeat", Name: "song a"}}
	if _, err := svc.SyncTracks(ctx, &orig, "tracker1"); err != nil {
		t.Fatalf("seed sync: %v", err)
	}

	renamed := []model.Track{{Id: "aaa", Artist: "yeat", Name: "song a fixed"}}
	res, err := svc.SyncTracks(ctx, &renamed, "tracker1")
	if err != nil {
		t.Fatalf("rename sync: %v", err)
	}
	if res.InsertedOrUpdated != 1 || len(res.TracksToDownload) != 1 {
		t.Fatalf("rename sync = %+v, want 1 requeued", res)
	}
}

func TestSyncTracksSameTitleDifferentIdKept(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	tracks := []model.Track{
		{Id: "id-one-000000000000000000000001", Artist: "yeat", Name: "headless"},
		{Id: "id-two-000000000000000000000002", Artist: "yeat", Name: "headless"},
	}
	res, err := svc.SyncTracks(ctx, &tracks, "tracker1")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.InsertedOrUpdated != 2 {
		t.Fatalf("inserted = %d, want 2 (same title, distinct ids)", res.InsertedOrUpdated)
	}
}

