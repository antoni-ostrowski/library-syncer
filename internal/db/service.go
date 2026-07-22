package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/antoni-ostrowski/library-syncer/internal/parser"
)

type DbService struct {
	db *sql.DB
}

func NewDbService(db *sql.DB) *DbService {
	return &DbService{db: db}
}

type SyncResult struct {
	insertedOrUpdated int
	deletionsCount    int
}

func (s SyncResult) String() string {
	return fmt.Sprintf("updated or updated: %v, deleted: %v", s.insertedOrUpdated, s.deletionsCount)
}

func (d *DbService) SyncTracks(ctx context.Context, sourceTracks *[]parser.Track) (SyncResult, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return SyncResult{}, err
	}

	defer tx.Rollback()

	result := SyncResult{}

	freshIds := make(map[string]struct{})

	for _, t := range *sourceTracks {

		hashId, jsonStr, err := prepareTrack(&t)
		if err != nil {
			return SyncResult{}, fmt.Errorf("failed to prepare track: %v", err)
		}

		upsertSQL := `
			INSERT INTO tracks (id, metadata)
			VALUES (?, ?)
			ON CONFLICT(id) DO UPDATE SET metadata = EXCLUDED.metadata
			WHERE tracks.metadata <> EXCLUDED.metadata;
		`

		if _, err := tx.ExecContext(ctx, upsertSQL, hashId, jsonStr); err != nil {
			return SyncResult{}, err
		}

		var changed int
		if err := tx.QueryRowContext(ctx, "SELECT changes();").Scan(&changed); err != nil {
			return SyncResult{}, err
		}

		if changed == 0 {
		} else {
			result.insertedOrUpdated++
		}

		freshIds[hashId] = struct{}{}
	}

	rows, err := tx.QueryContext(ctx, "SELECT id FROM tracks;")
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
			if _, err := tx.ExecContext(ctx, "DELETE FROM tracks WHERE id = ?;", dbId); err != nil {
				return SyncResult{}, err
			}
			result.deletionsCount++
		}
	}

	return result, tx.Commit()
}

func prepareTrack(track *parser.Track) (string, string, error) {
	jsonBytes, err := json.Marshal(track)
	if err != nil {
		return "", "", err
	}

	jsonString := string(jsonBytes)

	hash := sha256.Sum256([]byte(jsonString))
	// [:] to turn fixed size [32]byte (hash var) arr to slice []byte
	hashId := hex.EncodeToString(hash[:])
	return hashId, jsonString, nil
}
