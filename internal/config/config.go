package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultDbPath      = "/app/data/db"
	DefaultSecretsPath = "/app/data/secrets"
	DefaultSheetsPath  = "/app/sheets"
	DefaultAssetsPath  = "/app/assets/covers"
)

func DbPath() string {
	var p = os.Getenv("DB_PATH")
	if len(p) > 1 {
		return p
	}
	return DefaultDbPath
}

func SecretsPath() string {
	var p = os.Getenv("SECRETS_PATH")
	if len(p) > 1 {
		return p
	}
	return DefaultSecretsPath
}

func SheetsPath() string {
	var p = os.Getenv("SHEETS_PATH")
	if len(p) > 1 {
		return p
	}
	return DefaultSheetsPath
}

func AssetsPath() string {
	var p = os.Getenv("ASSETS_PATH")
	if len(p) > 1 {
		return p
	}
	return DefaultAssetsPath
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
func LoadEnv(filepath string) {
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
func ClearSheetsDir(dir string) {
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
