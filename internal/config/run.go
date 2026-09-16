package config

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
)

// App carries startup runtime values. DB is *sql.DB (not *db.DbService)
// so config never imports db and no import cycle is possible.
// Callers wrap it: db.NewDbService(cfg.DB).
type App struct {
	SleepSec int
	DevMode  bool
	DB       *sql.DB
	SongsDir string
}

func RunConfig() App {
	LoadEnv(".env.local")
	requiredEnvs := []string{
		"SONGS_PATH",
		"WORKER_COUNT",
		"SLEEP_SEC",
	}
	sleepSec, err := strconv.Atoi(os.Getenv("SLEEP_SEC"))
	if err != nil {
		fmt.Printf("Startup Error: incorrect sleep sec env value, expected number: %v\n", err)
		os.Exit(1)
	}

	var trackOutputDir = os.Getenv("SONGS_PATH")

	if err := ValidateEnvs(requiredEnvs); err != nil {
		fmt.Printf("Startup Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Environment configuration loaded successfully.")

	devMode := flag.Bool("d", false, "dev mode (only download sample size + 1 loop iteration)")
	flag.Parse()

	dbConn, err := OpenDb()
	if err != nil {
		log.Fatalf("failed to connect to database: %v\n", err.Error())
	}

	ClearSheetsDir(SheetsPath())
	toCreate := []string{trackOutputDir, SecretsPath(), SheetsPath()}

	for _, v := range toCreate {
		if err := os.MkdirAll(v, 0755); err != nil {
			log.Fatalf("failed to create dir: %v", err)
		}

	}

	fmt.Printf("dev mode %v\n", *devMode)

	return App{
		SleepSec: sleepSec,
		DevMode:  *devMode,
		DB:       dbConn,
		SongsDir: trackOutputDir,
	}
}
