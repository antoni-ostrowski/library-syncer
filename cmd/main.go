package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/antoni-ostrowski/library-syncer/internal/db"
	"github.com/antoni-ostrowski/library-syncer/internal/downloader"
	srccsv "github.com/antoni-ostrowski/library-syncer/internal/gsh"
	"github.com/antoni-ostrowski/library-syncer/internal/parser"
	"github.com/antoni-ostrowski/library-syncer/internal/syncer"
)

var trackOutputDir = os.Getenv("RSYNC_SRC")

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
