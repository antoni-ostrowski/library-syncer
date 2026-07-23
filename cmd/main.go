package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/antoni-ostrowski/library-syncer/internal/db"
	"github.com/antoni-ostrowski/library-syncer/internal/downloader"
	srccsv "github.com/antoni-ostrowski/library-syncer/internal/gsh"
	"github.com/antoni-ostrowski/library-syncer/internal/parser"
	"github.com/antoni-ostrowski/library-syncer/internal/syncer"
)

func main() {
	loadEnv(".env.local")

	requiredEnvs := []string{
		"RSYNC_USER",
		"RSYNC_HOSTNAME",
		"RSYNC_DEST",
		"SSH_KEY",
		"DB_PATH",
		"SONGS_PATH",
		"SECRETS_PATH",
		"WORKER_COUNT",
		"ASSETS_PATH",
	}

	if err := ValidateEnvs(requiredEnvs); err != nil {
		fmt.Printf("Startup Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Environment configuration loaded successfully.")

	devMode := flag.Bool("d", false, "dev mode (only download sample size + 1 loop iteration)")
	flag.Parse()
	var trackOutputDir = os.Getenv("SONGS_PATH")

	toCreate := []string{trackOutputDir, os.Getenv("SECRETS_PATH")}

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

	// first run to let the auth flow (pasting code to stdin)
	_, err = srccsv.DownloadSourceCsv(context.Background())
	if err != nil {
		fmt.Printf("failed to download source csv: %v\n", err)
		if *devMode {
			return
		}
		return
	}

	go func() {
		for {

			fmt.Printf("---executing the main loop... \n")
			ctx := context.WithValue(context.Background(), "devMode", *devMode)
			fmt.Printf("---downloading source csv file... \n")
			csvPath, err := srccsv.DownloadSourceCsv(ctx)
			if err != nil {
				fmt.Printf("failed to download source csv: %v\n", err)
				if *devMode {
					break
				}
				continue
			}
			fmt.Printf("csv at %v\n", csvPath)

			fmt.Printf("---parsing source csv file... \n")
			sourceTracks, err := parser.Parse(csvPath, trackOutputDir)
			if err != nil {
				fmt.Printf("failed to parse source csv: %v\n", err)
				if *devMode {
					break
				}
				continue
			}
			fmt.Printf("we have %v source tracks\n", len(sourceTracks))

			fmt.Printf("---syncing source tracks to database... \n")
			syncResult, err := db.SyncTracks(ctx, &sourceTracks)
			if err != nil {
				fmt.Printf("failed to sync tracks to db: %v\n", err)
				if *devMode {
					break
				}
				continue
			}
			fmt.Println(syncResult)

			fmt.Printf("---downloading tracks missing tracks... \n")
			downloader.DownloadTracks(ctx, &sourceTracks, trackOutputDir)

			fmt.Printf("---syncing files to client... \n")
			err = syncer.SyncFiles()
			if err != nil {
				fmt.Printf("failed to sync files: %v\n", err)
				if *devMode {
					break
				}
				continue
			}

			if *devMode {
				break
			}

			fmt.Printf("---sleeping... \n")
			time.Sleep(time.Second * 2)
		}

	}()

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalln("server error: ", err)
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
