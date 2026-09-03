package config

import "os"

const (
	DefaultSecretsPath = "/app/data/secrets"
	DefaultSheetsPath  = "/app/sheets"
	DefaultAssetsPath  = "/app/assets/covers"
)

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
