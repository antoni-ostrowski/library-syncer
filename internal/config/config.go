package config

import "os"

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
