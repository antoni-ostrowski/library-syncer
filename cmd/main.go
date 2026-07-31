package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/antoni-ostrowski/library-syncer/internal/db"
	"github.com/antoni-ostrowski/library-syncer/internal/downloader"
	srccsv "github.com/antoni-ostrowski/library-syncer/internal/gsh"
	"github.com/antoni-ostrowski/library-syncer/internal/parser"
	"github.com/antoni-ostrowski/library-syncer/internal/web"
)

func main() {
	loadEnv(".env.local")

	requiredEnvs := []string{
		"DB_PATH",
		"SONGS_PATH",
		"SECRETS_PATH",
		"WORKER_COUNT",
		"ASSETS_PATH",
		"SLEEP_SEC",
		"SHEETS_PATH",
	}

	sleepSec, err := strconv.Atoi(os.Getenv("SLEEP_SEC"))
	if err != nil {
		fmt.Printf("Startup Error: incorrect sleep sec env value, expected number: %v\n", err)
		os.Exit(1)
	}

	if err := ValidateEnvs(requiredEnvs); err != nil {
		fmt.Printf("Startup Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Environment configuration loaded successfully.")

	devMode := flag.Bool("d", false, "dev mode (only download sample size + 1 loop iteration)")
	flag.Parse()

	dbConn, err := db.OpenDb()
	if err != nil {
		log.Fatalf("failed to connect to database: %v\n", err.Error())
	}

	db := db.NewDbService(dbConn)

	go web.StartHttpServer(db)

	var trackOutputDir = os.Getenv("SONGS_PATH")

	clearSheetsDir(os.Getenv("SHEETS_PATH"))
	toCreate := []string{trackOutputDir, os.Getenv("SECRETS_PATH"), os.Getenv("SHEETS_PATH")}

	for _, v := range toCreate {
		if err := os.MkdirAll(v, 0755); err != nil {
			log.Fatalf("failed to create dir: %v", err)
		}

	}

	fmt.Printf("dev mode %v\n", *devMode)

	ctx := context.Background()
	trackers, err := db.ListTrackers(ctx)
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}

	tracksToDownload := make(chan downloader.Downloadable, 10000)

	for {
		fmt.Printf("---executing the main loop... \n")

		ctx := context.Background()
		downloader.DownloadTracks(ctx, *devMode, tracksToDownload)
		for _, v := range trackers {
			tracker := v
			tracker.Status = "syncing"
			if err := db.UpsertTracker(ctx, tracker); err != nil {
				log.Printf("failed to mark syncing: %v", err)
				continue
			}

			ExecuteTracker(ctx, db, v, tracksToDownload)

			tracker.Status = "synced"
			if err := db.UpsertTracker(ctx, tracker); err != nil {
				log.Printf("failed to mark final status: %v", err)
			}
		}

		fmt.Printf("---sleeping... \n")
		time.Sleep(time.Second * time.Duration(sleepSec))
	}

}

func ExecuteTracker(ctx context.Context, db *db.DbService, tracker parser.Tracker, tracksToDownload chan<- downloader.Downloadable) {
	fmt.Printf("running for %v\n", tracker.Artist)
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

}

func ValidateEnvs(required []string) error {
	var missing []string

	for _, env := range required {
		if strings.TrimSpace(os.Getenv(env)) == "" {
			missing = append(missing, env)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables:\n  - %s", strings.Join(missing, "\n  - "))
	}

	return nil
}
func loadEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}
func clearSheetsDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if err := os.RemoveAll(path); err != nil {
			fmt.Printf("failed to remove %s: %v", path, err)
		}
	}
}
