package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/antoni-ostrowski/library-syncer/internal/db"
	"github.com/antoni-ostrowski/library-syncer/internal/downloader"
	srccsv "github.com/antoni-ostrowski/library-syncer/internal/gsh"
	"github.com/antoni-ostrowski/library-syncer/internal/parser"
)

const trackOutputDir = "dev-output"

func main() {
	devMode := flag.Bool("d", false, "dev mode (only download sample size)")
	flag.Parse()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello")
	})
	fmt.Printf("dev mode %v\n", *devMode)

	dbConn, err := db.OpenDb()
	if err != nil {
		log.Fatalf("failed to connect to database: %v\n", err.Error())
	}

	db := db.NewDbService(dbConn)
	_ = db

	go func() {
		for {
			fmt.Printf("---executing the main loop... \n")
			ctx := context.WithValue(context.Background(), "devMode", *devMode)
			csvPath, err := srccsv.DownloadSourceCsv(ctx)
			if err != nil {
				fmt.Printf("failed to download source csv: %v\n", err)
				continue
			}
			fmt.Printf("csv at %v\n", csvPath)

			sourceTracks, err := parser.Parse(csvPath, trackOutputDir)
			if err != nil {
				fmt.Printf("failed to parse source csv: %v\n", err)
				continue
			}
			fmt.Printf("we have %v source tracks\n", len(sourceTracks))

			syncResult, err := db.SyncTracks(ctx, &sourceTracks)
			if err != nil {
				fmt.Printf("failed to sync tracks: %v\n", err)
			}
			fmt.Println(syncResult)

			downloader.DownloadTracks(ctx, &sourceTracks, trackOutputDir)

			if *devMode {
				break
			}

			time.Sleep(time.Second * 2)
		}

	}()

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalln("server error: ", err)
	}

}
