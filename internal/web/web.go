package web

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/antoni-ostrowski/library-syncer/internal/db"
	"github.com/antoni-ostrowski/library-syncer/internal/parser"
	"github.com/antoni-ostrowski/library-syncer/internal/runner"
	"github.com/antoni-ostrowski/library-syncer/internal/web/views"
	"go.senan.xyz/taglib"
)

func StartHttpServer(db *db.DbService, run *runner.Runner) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		trackers, err := db.ListTrackers(r.Context())
		if err != nil {
			views.Error(err.Error()).Render(r.Context(), w)
		}
		views.Base(views.Index(views.IndexModel{Trackers: trackers})).Render(r.Context(), w)
	})

	http.HandleFunc("GET /tracker-list", func(w http.ResponseWriter, r *http.Request) {
		trackers, err := db.ListTrackers(r.Context())
		if err != nil {
			views.Error(err.Error()).Render(r.Context(), w)
		}
		running := run.IsRunning()

		views.TrackerList(trackers, running).Render(r.Context(), w)
	})

	http.HandleFunc("POST /run", func(w http.ResponseWriter, r *http.Request) {
		run.Trigger(runner.Cmd{Type: runner.CmdTypeRunAll})
		w.Header().Set("HX-Trigger", "refreshList, runnerStateChanged")
		w.WriteHeader(http.StatusAccepted)
	})

	http.HandleFunc("POST /run/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		run.Trigger(runner.Cmd{Type: runner.CmdTypeRunOne, Id: id})
		w.Header().Set("HX-Trigger", "refreshList, runnerStateChanged")
		w.WriteHeader(http.StatusAccepted)
	})

	http.HandleFunc("/runner-state", func(w http.ResponseWriter, r *http.Request) {
		running := run.IsRunning()

		views.TriggerBtn(running).Render(r.Context(), w)
	})
	http.HandleFunc("POST /nuke-library", func(w http.ResponseWriter, r *http.Request) {
		NukeLibrary()
		os.Exit(0)
	})

	http.HandleFunc("POST /tracker", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		ranges := r.Form["range"]
		var readRanges []parser.ReadRange
		for i := range ranges {
			readRanges = append(readRanges, parser.ReadRange{
				Name: ranges[i],
				Mapping: parser.TrackerMapping{
					Name:  r.Form["mappingName"][i],
					Era:   r.Form["mappingEra"][i],
					Notes: r.Form["mappingNotes"][i],
					Links: r.Form["mappingLinks"][i],
				},
			})
		}

		spreadsheetId, ok := sheetID(r.FormValue("id"))
		if !ok {
			fmt.Printf("no spreadsheetId found")
			http.Error(w, "no spreadsheetId found in link", http.StatusBadRequest)

		}
		newTracker := parser.NewTracker(r.FormValue("artist"), spreadsheetId, "idle", readRanges)
		if err := db.UpsertTracker(r.Context(), newTracker); err != nil {
			fmt.Printf("upsert tracker failed: %v\n", err)
			http.Error(w, "failed to save tracker", http.StatusInternalServerError)
			return
		}
		running := run.IsRunning()
		views.Tracker(newTracker, running).Render(r.Context(), w)
	})
	http.HandleFunc("DELETE /tracker/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			fmt.Printf("no tracker id provided\n")
			http.Error(w, "failed to delete tracker: no id provided", http.StatusBadRequest)
			return
		}
		artistName, err := db.DeleteTracker(r.Context(), id)
		if err != nil {
			fmt.Printf("tracker deletion failed: %v\n", err)
			http.Error(w, "failed to delete tracker", http.StatusInternalServerError)
			return
		}
		_, err = DeleteTracksByArtist(artistName)
		if err != nil {
			fmt.Printf("song files deletion failed: %v\n", err)
			http.Error(w, "failed to delete songs", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("POST /tracker/{id}/reset", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no tracker id provided", http.StatusBadRequest)
			return
		}
		tracker, err := db.GetTracker(r.Context(), id)
		if err != nil {
			fmt.Printf("reset: failed to get tracker %s: %v\n", id, err)
			http.Error(w, "tracker not found", http.StatusNotFound)
			return
		}
		deleted, err := DeleteTracksByArtist(tracker.Artist)
		if err != nil {
			fmt.Printf("reset: song files deletion failed: %v\n", err)
			http.Error(w, "failed to delete songs", http.StatusInternalServerError)
			return
		}
		fmt.Printf("reset: deleted %d files for artist %s\n", len(deleted), tracker.Artist)

		tracks, err := db.GetTracksForTracker(r.Context(), id)
		if err != nil {
			fmt.Printf("reset: failed to load tracks for %s: %v\n", id, err)
			http.Error(w, "failed to load tracks", http.StatusInternalServerError)
			return
		}
		if len(tracks) == 0 {
			fmt.Printf("reset: no tracks in DB for %s — nothing to requeue\n", id)
			w.WriteHeader(http.StatusOK)
			return
		}
		n := run.Enqueue(tracks)
		fmt.Printf("reset: requeued %d/%d tracks for %s\n", n, len(tracks), tracker.Artist)
		w.Header().Set("HX-Trigger", "refreshList")
		w.WriteHeader(http.StatusAccepted)
	})

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalln("server error: ", err)
	}
}

func sheetID(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "spreadsheets" && parts[1] == "d" {
		return parts[2], true
	}
	return "", false
}

func NukeLibrary() {
	songsDir := os.Getenv("SONGS_PATH")
	if songsDir == "" {
		log.Fatal("SONGS_PATH not set")
	}

	dbDir := os.Getenv("DB_PATH")
	if dbDir == "" {
		log.Fatal("DB_PATH not set")
	}

	entries, err := os.ReadDir(songsDir)
	if err != nil {
		log.Fatalf("failed to read songs dir: %v", err)
	}

	for _, entry := range entries {
		path := filepath.Join(songsDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			log.Fatalf("failed to remove %s: %v", path, err)
		}
	}

	dbFile := filepath.Join(dbDir, "data.db")

	os.Remove(dbFile)
	os.Remove(dbFile + "-wal")
	os.Remove(dbFile + "-shm")
}

func DeleteTracksByArtist(artist string) ([]string, error) {
	songsDir := os.Getenv("SONGS_PATH")
	if songsDir == "" {
		return nil, fmt.Errorf("SONGS_PATH not set")
	}

	var deleted []string
	err := filepath.WalkDir(songsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".mp3" && ext != ".flac" && ext != ".m4a" && ext != ".ogg" {
			return nil
		}

		tags, err := taglib.ReadTags(path)
		if err != nil {
			// skip unreadable files, or return err if you want strict
			return nil
		}

		for _, a := range tags[taglib.Artist] {
			if strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(artist)) {
				if err := os.Remove(path); err != nil {
					return fmt.Errorf("failed to remove %s: %w", path, err)
				}
				deleted = append(deleted, path)
				break
			}
		}

		return nil
	})

	return deleted, err
}
