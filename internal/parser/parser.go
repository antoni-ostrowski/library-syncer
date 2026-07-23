package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/gocarina/gocsv"
)

type Track struct {
	Era            string `csv:"Era"`
	Name           string `csv:"Name"`
	Notes          string `csv:"Notes\n(Join the Yeat Hub Discord!)"`
	FileDate       string `csv:"File Date"`
	Type           string `csv:"Type"`
	AvailableLen   string `csv:"Available Length"`
	Quality        string `csv:"Quality"`
	Links          string `csv:"Link(s)"`
	FirstPreview   string `csv:"First Preview"`
	LeakDate       string `csv:"Leak Date"`
	OGFileLeakDate string `csv:"OG File Leak Date"`
	RealLinks      []string
	OutputFilePath string
}

// fixes something with trailing commas. changes behaviour of encoding/csv in whole process
func init() {
	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		r.FieldsPerRecord = -1
		return r
	})
}

func Parse(csvPath string, trackOutputDir string) ([]Track, error) {
	tracksFile, err := os.OpenFile(csvPath, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open file")
	}

	fmt.Println("opened csv file")

	defer tracksFile.Close()

	allRows := []Track{}

	if err := gocsv.UnmarshalFile(tracksFile, &allRows); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %v", err)
	}

	var cleanedTracks []Track
	for i := range allRows {
		track := &allRows[i]
		links := getTracksLinks(*track)
		if len(links) == 0 {
			continue
		}
		track.Name = strings.Join(strings.Fields(track.Name), " ")
		track.OutputFilePath = path.Join(trackOutputDir, track.Name)
		track.RealLinks = links
		cleanedTracks = append(cleanedTracks, *track)
	}

	return cleanedTracks, nil

}

func getTracksLinks(track Track) []string {
	if strings.EqualFold(strings.TrimSpace(track.Links), "Source Needed") {
		return []string{}
	}

	links := strings.Fields(track.Links)
	links = slices.DeleteFunc(links, func(s string) bool {
		lowerS := strings.ToLower(strings.TrimSpace(s))

		// 1. If it's NOT from pillows.su, delete it.
		if !strings.Contains(lowerS, "pillows.su") {
			return true
		}

		// 2. If it explicitly ends in .jpg, delete it.
		if strings.HasSuffix(lowerS, ".jpg") {
			return true
		}

		// Otherwise, keep it (these are your /api/download/ID links)
		return false
	})

	return links
}
