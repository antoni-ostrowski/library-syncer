package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

func OpenDb() (*sql.DB, error) {
	fmt.Println("opening conn to db...")
	dbPath := os.Getenv("DB_PATH")

	if err := os.MkdirAll(dbPath, 0755); err != nil {
		log.Fatalf("failed to create db dir %v", err)
	}

	db, err := sql.Open("sqlite", path.Join(dbPath, "data.db?_journal_mode=WAL"))
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	fmt.Println("executing schema...")
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	fmt.Println("db conn successfull")
	return db, nil
}
