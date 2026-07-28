package parser

import (
	"encoding/csv"
	"io"
	"os"
	"strings"

	"github.com/antoni-ostrowski/library-syncer/internal/downloader"
)

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

func Parse(csvPath string, tracker Tracker) ([]downloader.Downloadable, error) {
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

	var downloadables []downloader.Downloadable
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

		track := downloader.Track{
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

		track.Name = strings.Join(strings.Fields(track.Name), " ")

		linksAr := strings.Fields(track.Links)
		for _, link := range linksAr {
			if strings.Contains(link, "pillows.su") {
				a := &downloader.DownloadableTrack{Track: track, Url: createPillowcaseLink(link), Source: downloader.SourcePillowcase}
				downloadables = append(downloadables, a)
				continue
			}

			if strings.Contains(link, "soundcloud.com") {
				a := &downloader.DownloadableTrack{Track: track, Url: link, Source: downloader.SourceSc}
				downloadables = append(downloadables, a)
				continue
			}

		}

	}

	return downloadables, nil
}

func createPillowcaseLink(link string) string {
	const baseApiUrl = "https://api.pillows.su"
	const downloadEndpoint = "/api/download/"
	var trackId string
	if len(link) >= 32 {
		trackId = link[len(link)-32:]
	} else {
		return ""
	}

	downloadLink := baseApiUrl + downloadEndpoint + trackId
	return downloadLink
}
