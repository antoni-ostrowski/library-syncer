package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/antoni-ostrowski/library-syncer/internal/model"
)

type DbService struct {
	db *sql.DB
}

func NewDbService(db *sql.DB) *DbService {
	return &DbService{db: db}
}

type SyncResult struct {
	InsertedOrUpdated int
	DeletionsCount    int
	TracksToDownload  []model.Track
}

func (s SyncResult) String() string {
	return fmt.Sprintf("inserted or updated: %v, deleted: %v", s.InsertedOrUpdated, s.DeletionsCount)
}

func (d *DbService) SyncTracks(ctx context.Context, sourceTracks *[]model.Track, tracker_id string) (SyncResult, error) {
	fmt.Printf("---syncing source tracks to database... \n")

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return SyncResult{}, err
	}

	defer tx.Rollback()

	result := SyncResult{}

	freshIds := make(map[string]struct{})

	for _, t := range *sourceTracks {
		jsonBytes, err := json.Marshal(t)
		if err != nil {
			return SyncResult{}, fmt.Errorf("failed to prepare track: %v", err)
		}

		upsertSQL := `
			INSERT INTO tracks (pillow_id, tracker_id, metadata)
			VALUES (?, ?, ?)
			ON CONFLICT(pillow_id, tracker_id) DO UPDATE SET metadata = EXCLUDED.metadata
			WHERE tracks.metadata <> EXCLUDED.metadata;
		`

		if _, err := tx.ExecContext(ctx, upsertSQL, t.Id, tracker_id, string(jsonBytes)); err != nil {
			return SyncResult{}, err
		}

		var changed int
		if err := tx.QueryRowContext(ctx, "SELECT changes();").Scan(&changed); err != nil {
			return SyncResult{}, err
		}

		if changed == 0 {
		} else {
			result.InsertedOrUpdated++
			result.TracksToDownload = append(result.TracksToDownload, t)
		}

		freshIds[t.Id] = struct{}{}
	}

	rows, err := tx.QueryContext(ctx, "SELECT pillow_id FROM tracks WHERE tracker_id = ?;", tracker_id)
	if err != nil {
		return SyncResult{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var dbId string
		if err := rows.Scan(&dbId); err != nil {
			return SyncResult{}, err
		}
		if _, exists := freshIds[dbId]; !exists {
			if _, err := tx.ExecContext(ctx, "DELETE FROM tracks WHERE pillow_id = ? AND tracker_id = ?;", dbId, tracker_id); err != nil {
				return SyncResult{}, err
			}
			result.DeletionsCount++
		}
	}

	return result, tx.Commit()
}

func (d *DbService) ListTrackers(ctx context.Context) ([]model.Tracker, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT id, read_ranges, artist, status FROM trackers;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trackers []model.Tracker
	for rows.Next() {
		var t model.Tracker
		var readRangesJSON string
		if err := rows.Scan(&t.Id, &readRangesJSON, &t.Artist, &t.Status); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(readRangesJSON), &t.ReadRanges); err != nil {
			return nil, err
		}

		trackers = append(trackers, t)
	}

	return trackers, rows.Err()

}

func (d *DbService) UpsertTracker(ctx context.Context, newTracker model.Tracker) error {
	readRangesJSON, err := json.Marshal(newTracker.ReadRanges)
	if err != nil {
		return err
	}

	_, err = d.db.ExecContext(ctx, `
		INSERT INTO trackers (id, read_ranges, artist, status)
		VALUES (?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
		read_ranges = EXCLUDED.read_ranges,
		artist = EXCLUDED.artist,
		status = EXCLUDED.status;
		`,
		newTracker.Id, readRangesJSON, newTracker.Artist, newTracker.Status)

	return err
}

func (d *DbService) DeleteTracker(ctx context.Context, trackerId string) (string, error) {
	tracker, err := d.GetTracker(ctx, trackerId)
	if err != nil {
		return "", err
	}
	if _, err := d.db.ExecContext(ctx, `DELETE FROM trackers WHERE id = ?;`, trackerId); err != nil {
		return "", err
	}
	if _, err := d.db.ExecContext(ctx, `DELETE FROM tracks WHERE tracker_id = ?;`, trackerId); err != nil {
		return "", err
	}

	return tracker.Artist, nil

}

func (d *DbService) GetTracker(ctx context.Context, trackerId string) (model.Tracker, error) {
	var t model.Tracker
	var readRangesJSON string
	err := d.db.QueryRowContext(ctx, `SELECT id, read_ranges, artist, status FROM trackers WHERE id = ?;`, trackerId).Scan(&t.Id, &readRangesJSON, &t.Artist, &t.Status)
	if err == sql.ErrNoRows {
		return t, err
	}
	if err != nil {
		return t, err
	}

	if err := json.Unmarshal([]byte(readRangesJSON), &t.ReadRanges); err != nil {
		return t, err
	}

	return t, err

}

func (d *DbService) GetTracksForTracker(ctx context.Context, trackerId string) ([]model.Track, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT metadata FROM tracks WHERE tracker_id = ?;`, trackerId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.Track
	for rows.Next() {
		var meta string
		if err := rows.Scan(&meta); err != nil {
			return nil, err
		}
		var dt model.Track
		if err := json.Unmarshal([]byte(meta), &dt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal track metadata: %w", err)
		}
		// copy to heap so each pointer is distinct
		track := dt
		result = append(result, track)
	}
	return result, rows.Err()
}

