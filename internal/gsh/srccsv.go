package srccsv

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	credentialsPath = "./data/secrets/credentials.json"
	tokenPath       = "./data/secrets/token.json"
	spreadsheetID   = "1FUzAZyTCgFTVxQ--qbCAS2bUk4dsAw6ASxwjURPHbyI"
	readRange       = "Unreleased"
	outputPath      = "sheet.csv"
)

func DownloadSourceCsv(ctx context.Context) (string, error) {

	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		log.Printf("unable to read credentials: %v", err)
		return "", err
	}

	config, err := google.ConfigFromJSON(b, sheets.SpreadsheetsReadonlyScope)
	if err != nil {
		log.Printf("unable to parse credentials: %v", err)
		return "", err
	}

	// Must match an authorized redirect URI in Google Cloud Console.
	config.RedirectURL = "http://localhost"

	client := getClient(ctx, config)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Printf("unable to create sheets service: %v", err)
		return "", err
	}

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("unable to retrieve data: %v", err)
		return "", err
	}

	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
		return "", nil
	}

	if err := writeCSV(outputPath, resp.Values); err != nil {
		log.Printf("unable to write csv: %v", err)
		return "", err
	}

	fmt.Printf("wrote %d rows to %s\n", len(resp.Values), outputPath)
	return outputPath, nil
}

func writeCSV(path string, rows [][]any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	for _, row := range rows {
		record := make([]string, len(row))
		for i, v := range row {
			record[i] = fmt.Sprint(v)
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	tok, err := tokenFromFile(tokenPath)
	if err != nil {
		tok = getTokenFromWeb(ctx, config)
		saveToken(tokenPath, tok)
	}
	return config.Client(ctx, tok)
}

func getTokenFromWeb(ctx context.Context, config *oauth2.Config) *oauth2.Token {
	path := "./data/secrets/code.txt"
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	fmt.Printf("Go to this URL in your browser and authorize the app:\n%s\n", authURL)
	fmt.Printf("After authorization, write the code from the URL to %s\n", path)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Fatalf("timeout waiting for authorization code")
		case <-ticker.C:
		}

		fmt.Printf("--- reading code from %s\n", path)
		b, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				fmt.Printf("failed to read code file: %v\n", err)
			}
			continue
		}

		code := strings.TrimSpace(string(b))
		if code == "" {
			fmt.Printf("code file is empty, waiting...\n")
			continue
		}

		tok, err := config.Exchange(ctx, code)
		if err != nil {
			log.Fatalf("unable to exchange token: %v", err)
		}

		if err := os.WriteFile(path, []byte{}, 0600); err != nil {
			fmt.Printf("warning: failed to clear code file: %v\n", err)
		}

		return tok
	}
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("saving token to %s\n", path)

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("unable to cache token: %v", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(token); err != nil {
		log.Fatalf("unable to encode token: %v", err)
	}
}
