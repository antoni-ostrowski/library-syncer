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

	"github.com/antoni-ostrowski/library-syncer/internal/config"
	"github.com/antoni-ostrowski/library-syncer/internal/db"
	"github.com/antoni-ostrowski/library-syncer/internal/runner"
	"github.com/antoni-ostrowski/library-syncer/internal/web"
)

func main() {
	sleepSec, devMode, db := runConfig()
	run := runner.New(db, sleepSec, devMode)
	ctx := context.Background()
	go run.Start(ctx)
	run.Trigger(runner.Cmd{Type: runner.CmdTypeRunAll})
	go web.StartHttpServer(db, run)
	select {}
}

func runConfig() (int, bool, *db.DbService) {
	loadEnv(".env.local")
	requiredEnvs := []string{
		"DB_PATH",
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

	dbConn, err := db.OpenDb()
	if err != nil {
		log.Fatalf("failed to connect to database: %v\n", err.Error())
	}

	db := db.NewDbService(dbConn)

	clearSheetsDir(config.SheetsPath())
	toCreate := []string{trackOutputDir, config.SecretsPath(), config.SheetsPath()}

	for _, v := range toCreate {
		if err := os.MkdirAll(v, 0755); err != nil {
			log.Fatalf("failed to create dir: %v", err)
		}

	}

	fmt.Printf("dev mode %v\n", *devMode)

	return sleepSec, *devMode, db

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
