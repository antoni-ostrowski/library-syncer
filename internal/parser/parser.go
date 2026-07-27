package parser

import (
	"encoding/csv"
	"io"
	"os"
	"path"
	"slices"
	"strings"
)

type Track struct {
	Artist         string
	Era            string
	Name           string
	Notes          string
	FileDate       string
	Type           string
	AvailableLen   string
	Quality        string
	Links          string
	FirstPreview   string
	LeakDate       string
	OGFileLeakDate string
	RealLinks      []string
	OutputFilePath string
}

type Tracker struct {
	Artist        string
	Mapping       TrackerMapping
	SpreadsheetID string
	ReadRange     []string
}

func NewTracker(artist string, spreadsheetID string, readRange []string, mapping TrackerMapping) Tracker {
	return Tracker{
		Artist:        artist,
		ReadRange:     readRange,
		SpreadsheetID: spreadsheetID,
		Mapping:       mapping,
	}
}

type TrackerMapping struct {
	Era            string
	Name           string
	Notes          string
	FileDate       string
	Type           string
	AvailableLen   string
	Quality        string
	Links          string
	FirstPreview   string
	LeakDate       string
	OGFileLeakDate string
}

func Parse(csvPath string, tracker Tracker) ([]Track, error) {
	var trackOutputDir = os.Getenv("SONGS_PATH")
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	headers, err := r.Read()
	if err != nil {
		return nil, err
	}

	headerIdx := map[string]int{}
	// map each column to int
	for i, h := range headers {
		headerIdx[h] = i
	}

	var tracks []Track
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		get := func(col string) string {
			// get the index of column, retrieve data from row with that index
			if i, ok := headerIdx[col]; ok && i < len(row) {
				return row[i]
			}
			return ""
		}

		track := Track{
			Artist:         tracker.Artist,
			Name:           get(tracker.Mapping.Name),
			Links:          get(tracker.Mapping.Links),
			Era:            get(tracker.Mapping.Era),
			Notes:          get(tracker.Mapping.Notes),
			FileDate:       get(tracker.Mapping.FileDate),
			Type:           get(tracker.Mapping.Type),
			AvailableLen:   get(tracker.Mapping.AvailableLen),
			Quality:        get(tracker.Mapping.Quality),
			FirstPreview:   get(tracker.Mapping.FirstPreview),
			LeakDate:       get(tracker.Mapping.LeakDate),
			OGFileLeakDate: get(tracker.Mapping.OGFileLeakDate),
		}

		links := getTracksLinks(track)
		if len(links) == 0 {
			continue
		}
		track.Name = strings.Join(strings.Fields(track.Name), " ")
		track.OutputFilePath = path.Join(trackOutputDir, track.Name)
		track.RealLinks = links

		tracks = append(tracks, track)
	}

	return tracks, nil
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
