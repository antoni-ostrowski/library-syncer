package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/antoni-ostrowski/library-syncer/internal/db"
	"github.com/antoni-ostrowski/library-syncer/internal/downloader"
	srccsv "github.com/antoni-ostrowski/library-syncer/internal/gsh"
	"github.com/antoni-ostrowski/library-syncer/internal/parser"
)

var yeatTracker = parser.NewTracker("yeat", "1FUzAZyTCgFTVxQ--qbCAS2bUk4dsAw6ASxwjURPHbyI", []string{"Released", "Unreleased"}, parser.TrackerMapping{
	Era:            "Era",
	Name:           "Name",
	Notes:          "Notes\n(Join the Yeat Hub Discord!)",
	FileDate:       "File Date",
	Type:           "Type",
	AvailableLen:   "Available Length",
	Quality:        "Quality",
	Links:          "Link(s)",
	FirstPreview:   "First Preview",
	LeakDate:       "Leak Date",
	OGFileLeakDate: "OG File Leak Date",
})

// ttrstrnsei
var masonTracker = parser.NewTracker("osamason", "1qbJpawdwnw7IUkZ4FfU3oOF17griNjL59X5z6YFiFlY", []string{"Unreleased!A2:J", "Released"}, parser.TrackerMapping{
	Era:            "Era",
	Name:           "Name",
	Notes:          "Notes",
	FileDate:       "File Date",
	Type:           "Type",
	AvailableLen:   "Track Length",
	Quality:        "Quality",
	Links:          "Link(s)",
	FirstPreview:   "First Preview",
	LeakDate:       "Leak Date",
	OGFileLeakDate: "OG File Leak Date",
})

var uziTracker = parser.NewTracker("uzi", "1zqqdIds1iwnx4lh29iF1IlraeuqfGhxH9qLNlWOnryo", []string{"💿 Unreleased", "📻 Released"}, parser.TrackerMapping{
	Era:            "Era",
	Name:           "Name ",
	Notes:          "Notes\n(Join the Discord Server!)",
	FileDate:       "File Date",
	Type:           "Type",
	AvailableLen:   "Track Length",
	Quality:        "Quality",
	Links:          "Links",
	FirstPreview:   "First Preview",
	LeakDate:       "Leak Date",
	OGFileLeakDate: "OG File Leak Date",
})

var cartiTracker = parser.NewTracker("carti", "1Irtfvymu26CShYowLMMfD-rM0o9CJqE6-BBSlYsAaF4", []string{"💿 Unreleased", "📻 Released"}, parser.TrackerMapping{
	Era:            "Era",
	Name:           "Name",
	Notes:          "Notes\nJoin our discord server here\nUse grails.cx/tracker to share our tracker!",
	FileDate:       "File Date",
	Type:           "Type",
	AvailableLen:   "Track Length",
	Quality:        "Quality",
	Links:          "Link(s)",
	FirstPreview:   "First Preview",
	LeakDate:       "Leak Date",
	OGFileLeakDate: "OG File Leak Date",
})

var edwardTracker = parser.NewTracker("edward skeletrix", "1CnfVdc37A81ZX7lUs4L-J2JfqaW2z3pbkR2W6dRuFS0", []string{"💿 Unreleased", "📀 Released"}, parser.TrackerMapping{
	Era:            "Era",
	Name:           "Name\n(Check out the ArtistGrid Website!)",
	Notes:          "Notes\n(Join the Edward Hub Discord!)",
	FileDate:       "File Date",
	Type:           "Type",
	AvailableLen:   "Track Length",
	Quality:        "Quality",
	Links:          "Link(s)",
	FirstPreview:   "First Preview",
	LeakDate:       "Leak Date",
	OGFileLeakDate: "OG File Leak Date",
})

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
	var trackOutputDir = os.Getenv("SONGS_PATH")

	clearSheetsDir(os.Getenv("SHEETS_PATH"))
	toCreate := []string{trackOutputDir, os.Getenv("SECRETS_PATH"), os.Getenv("SHEETS_PATH")}

	for _, v := range toCreate {
		if err := os.MkdirAll(v, 0755); err != nil {
			log.Fatalf("failed to create dir: %v", err)
		}

	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello")
	})

	fmt.Printf("dev mode %v\n", *devMode)

	dbConn, err := db.OpenDb()
	if err != nil {
		log.Fatalf("failed to connect to database: %v\n", err.Error())
	}

	db := db.NewDbService(dbConn)

	toPerform := []parser.Tracker{
		yeatTracker,
		masonTracker,
		uziTracker,
		cartiTracker,
		edwardTracker,
	}

	tracksToDownload := make(chan downloader.Downloadable, 10000)

	go func() {
		for {
			fmt.Printf("---executing the main loop... \n")

			ctx := context.Background()
			downloader.DownloadTracks(ctx, *devMode, tracksToDownload)
			for _, v := range toPerform {
				ExecuteTracker(ctx, db, v, tracksToDownload)
			}

			if *devMode {
				break
			}

			fmt.Printf("---sleeping... \n")
			time.Sleep(time.Second * time.Duration(sleepSec))
		}

	}()

	if err := http.ListenAndServe(":3000", nil); err != nil {
		close(tracksToDownload)
		log.Fatalln("server error: ", err)
	}

}

func ExecuteTracker(ctx context.Context, db *db.DbService, tracker parser.Tracker, tracksToDownload chan<- downloader.Downloadable) {
	fmt.Printf("running for %v\n", tracker.Artist)
	for _, readRange := range tracker.ReadRange {
		csvPath, err := srccsv.DownloadSourceCsv(ctx, tracker.SpreadsheetID, readRange)
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

		trackerId := tracker.SpreadsheetID + "#" + readRange
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
