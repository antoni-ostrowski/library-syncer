package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/antoni-ostrowski/library-syncer/internal/db"
	srccsv "github.com/antoni-ostrowski/library-syncer/internal/gsh"
)

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

	csvPath, err := srccsv.DownloadSourceCsv()
	if err != nil {
		log.Fatalln("failed to download source csv: ", err)
	}
	log.Printf("csv at %v", csvPath)

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalln("server error: ", err)
	}

}
