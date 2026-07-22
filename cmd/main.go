package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/antoni-ostrowski/library-syncer/internal/db"
	srccsv "github.com/antoni-ostrowski/library-syncer/internal/gsh"
	"github.com/antoni-ostrowski/library-syncer/internal/parser"
)

const trackOutputDir = "dev-output"

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello")
	})

	dbConn, err := db.OpenDb()
	if err != nil {
		log.Fatalf("failed to connect to database: %v\n", err.Error())
	}

	db := db.NewDbService(dbConn)
	_ = db

	go func() {
		for {
			fmt.Printf("---executing the main loop... \n")
			csvPath, err := srccsv.DownloadSourceCsv()
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

			for _, t := range sourceTracks {
				fmt.Printf("%+v\n", t)
			}

			time.Sleep(time.Second * 2)
		}

	}()

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalln("server error: ", err)
	}

}
