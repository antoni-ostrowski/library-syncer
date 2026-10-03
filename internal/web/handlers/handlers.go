package handlers

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/antoni-ostrowski/library-syncer/internal/config"
	"github.com/antoni-ostrowski/library-syncer/internal/db"
	"github.com/antoni-ostrowski/library-syncer/internal/model"
	"github.com/antoni-ostrowski/library-syncer/internal/runner"
	"github.com/antoni-ostrowski/library-syncer/internal/web/views"
	"go.senan.xyz/taglib"
)

func Register(mux *http.ServeMux, db *db.DbService, run *runner.Runner) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		trackers, err := db.ListTrackers(r.Context())
		if err != nil {
			views.Error(err.Error()).Render(r.Context(), w)
		}
		archives, err := db.ListStaticAssets(r.Context())
		if err != nil {
			views.Error(err.Error()).Render(r.Context(), w)
		}
		views.Base(views.Index(views.IndexModel{Trackers: trackers, Archives: archives})).Render(r.Context(), w)
	})

	mux.HandleFunc("GET /tracker-list", func(w http.ResponseWriter, r *http.Request) {
		trackers, err := db.ListTrackers(r.Context())
		if err != nil {
			views.Error(err.Error()).Render(r.Context(), w)
		}
		running := run.IsRunning()

		views.TrackerList(trackers, running).Render(r.Context(), w)
	})

	mux.HandleFunc("POST /run", func(w http.ResponseWriter, r *http.Request) {
		run.Trigger(runner.Cmd{Type: runner.CmdTypeRunAll})
		w.Header().Set("HX-Trigger", "refreshList, runnerStateChanged")
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("POST /run/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		run.Trigger(runner.Cmd{Type: runner.CmdTypeRunOne, Id: id})
		w.Header().Set("HX-Trigger", "refreshList, runnerStateChanged")
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("/runner-state", func(w http.ResponseWriter, r *http.Request) {
		running := run.IsRunning()

		views.TriggerBtn(running).Render(r.Context(), w)
	})
	mux.HandleFunc("POST /nuke-library", func(w http.ResponseWriter, r *http.Request) {
		NukeLibrary()
		os.Exit(0)
	})

	mux.HandleFunc("POST /tracker", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		ranges := r.Form["range"]
		var readRanges []model.ReadRange
		for i := range ranges {
			readRanges = append(readRanges, model.ReadRange{
				Name: ranges[i],
				Mapping: model.TrackerMapping{
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
		newTracker := model.NewTracker(r.FormValue("artist"), spreadsheetId, "idle", readRanges)
		if err := db.UpsertTracker(r.Context(), newTracker); err != nil {
			fmt.Printf("upsert tracker failed: %v\n", err)
			http.Error(w, "failed to save tracker", http.StatusInternalServerError)
			return
		}
		running := run.IsRunning()
		views.Tracker(newTracker, running).Render(r.Context(), w)
	})
	mux.HandleFunc("DELETE /tracker/{id}", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("POST /tracker/{id}/reset", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("GET /archive-list", func(w http.ResponseWriter, r *http.Request) {
		archives, err := db.ListStaticAssets(r.Context())
		if err != nil {
			views.Error(err.Error()).Render(r.Context(), w)
			return
		}
		views.StaticArchiveList(archives).Render(r.Context(), w)
	})

	mux.HandleFunc("POST /static-archives", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "failed to parse upload", http.StatusBadRequest)
			return
		}
		label := strings.TrimSpace(r.FormValue("label"))
		if label == "" {
			http.Error(w, "label is required", http.StatusBadRequest)
			return
		}
		zips := r.MultipartForm.File["zipfile"]
		if len(zips) == 0 {
			http.Error(w, "no zip file provided", http.StatusBadRequest)
			return
		}
		fh := zips[0]
		if !strings.HasSuffix(strings.ToLower(fh.Filename), ".zip") {
			http.Error(w, "only .zip files accepted", http.StatusBadRequest)
			return
		}

		src, err := fh.Open()
		if err != nil {
			http.Error(w, "failed to read upload", http.StatusBadRequest)
			return
		}
		defer src.Close()
		tmp, err := os.CreateTemp("", "archive-*.zip")
		if err != nil {
			http.Error(w, "failed to save upload", http.StatusInternalServerError)
			return
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		if _, err := io.Copy(tmp, src); err != nil {
			tmp.Close()
			http.Error(w, "failed to save upload", http.StatusInternalServerError)
			return
		}
		if err := tmp.Close(); err != nil {
			http.Error(w, "failed to save upload", http.StatusInternalServerError)
			return
		}

		zr, err := zip.OpenReader(tmpName)
		if err != nil {
			http.Error(w, "not a valid zip file", http.StatusBadRequest)
			return
		}
		defer zr.Close()

		names := make([]string, 0, len(zr.File))
		for _, f := range zr.File {
			names = append(names, f.Name)
		}
		fallback := strings.TrimSuffix(filepath.Base(fh.Filename), filepath.Ext(fh.Filename))
		top, kept, rels, dirs, ok := zipLayout(names, fallback)
		if !ok || len(kept) == 0 {
			http.Error(w, "zip has no usable files", http.StatusBadRequest)
			return
		}

		exists, err := db.StaticAssetExists(r.Context(), label, top)
		if err != nil {
			http.Error(w, "failed to check archives", http.StatusInternalServerError)
			return
		}
		if exists {
			http.Error(w, "static archive label or directory already exists", http.StatusConflict)
			return
		}

		songsDir := os.Getenv("SONGS_PATH")
		if songsDir == "" {
			http.Error(w, "SONGS_PATH not set", http.StatusInternalServerError)
			return
		}
		destRoot := filepath.Join(songsDir, top)
		if _, err := os.Stat(destRoot); err == nil {
			http.Error(w, "static archive directory already exists on disk", http.StatusConflict)
			return
		}

		if err := writeZipArchive(zr, kept, rels, dirs, destRoot); err != nil {
			os.RemoveAll(destRoot)
			http.Error(w, "failed to save archive", http.StatusInternalServerError)
			return
		}

		asset := model.StaticAsset{
			Id:        model.NewStaticAssetID(),
			Name:      label,
			Dir:       top,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := db.CreateStaticAsset(r.Context(), asset); err != nil {
			os.RemoveAll(destRoot)
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				http.Error(w, "static archive label or directory already exists", http.StatusConflict)
				return
			}
			http.Error(w, "failed to save archive", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	mux.HandleFunc("DELETE /static-archives/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no archive id provided", http.StatusBadRequest)
			return
		}
		asset, err := db.GetStaticAsset(r.Context(), id)
		if err != nil {
			if err == sql.ErrNoRows {
				w.WriteHeader(http.StatusOK)
				return
			}
			http.Error(w, "failed to load archive", http.StatusInternalServerError)
			return
		}
		if songsDir := os.Getenv("SONGS_PATH"); songsDir != "" {
			if err := os.RemoveAll(filepath.Join(songsDir, asset.Dir)); err != nil {
				http.Error(w, "failed to delete archive files", http.StatusInternalServerError)
				return
			}
		}
		if err := db.DeleteStaticAsset(r.Context(), id); err != nil {
			http.Error(w, "failed to delete archive", http.StatusInternalServerError)
			return
		}
		archives, err := db.ListStaticAssets(r.Context())
		if err != nil {
			http.Error(w, "failed to load archives", http.StatusInternalServerError)
			return
		}
		views.StaticArchiveList(archives).Render(r.Context(), w)
	})
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

// zipLayout maps zip entries onto the archive tree. Entries under one
// common top folder keep it as root; otherwise the zip's own basename is
// the root and entries land verbatim. macOS metadata (__MACOSX, .DS_Store)
// is skipped. kept holds indices into names aligned with rels; dirs holds
// explicit directory entries so empty folders survive too.
func zipLayout(names []string, fallbackTop string) (top string, kept []int, rels []string, dirs []string, ok bool) {
	shared := ""
	nested := true
	for _, name := range names {
		if isZipSkipped(name) || strings.HasSuffix(name, "/") {
			continue
		}
		segs := strings.Split(strings.ReplaceAll(name, "\\", "/"), "/")
		if len(segs) < 2 {
			nested = false
			break
		}
		if shared == "" {
			shared = segs[0]
		} else if segs[0] != shared {
			nested = false
			break
		}
	}
	if nested && shared != "" {
		top = shared
	} else {
		top = fallbackTop
	}
	if top == "" || top == "." || top == ".." {
		return "", nil, nil, nil, false
	}

	for i, name := range names {
		if strings.HasSuffix(name, "/") {
			segs := strings.Split(strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/"), "/")
			for _, s := range segs {
				if s == "" || s == "." || s == ".." {
					return "", nil, nil, nil, false
				}
			}
			rel := filepath.Join(segs...)
			if nested && len(segs) > 1 {
				rel = filepath.Join(segs[1:]...)
			} else if nested {
				continue
			}
			dirs = append(dirs, rel)
			continue
		}
		if isZipSkipped(name) {
			continue
		}
		segs := strings.Split(strings.ReplaceAll(name, "\\", "/"), "/")
		for _, s := range segs {
			if s == "" || s == "." || s == ".." {
				return "", nil, nil, nil, false
			}
		}
		rel := filepath.Join(segs...)
		if nested {
			rel = filepath.Join(segs[1:]...)
		}
		kept = append(kept, i)
		rels = append(rels, rel)
	}
	return top, kept, rels, dirs, true
}

func isZipSkipped(name string) bool {
	segs := strings.Split(strings.ReplaceAll(name, "\\", "/"), "/")
	if len(segs) > 0 && segs[0] == "__MACOSX" {
		return true
	}
	return segs[len(segs)-1] == ".DS_Store"
}

func writeZipArchive(zr *zip.ReadCloser, kept []int, rels []string, dirs []string, destRoot string) error {
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(destRoot, d), 0755); err != nil {
			return err
		}
	}
	for i, idx := range kept {
		rc, err := zr.File[idx].Open()
		if err != nil {
			return err
		}
		dest := filepath.Join(destRoot, rels[i])
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			rc.Close()
			return err
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func NukeLibrary() {
	songsDir := os.Getenv("SONGS_PATH")
	if songsDir == "" {
		log.Fatal("SONGS_PATH not set")
	}

	dbDir := config.DbPath()

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
