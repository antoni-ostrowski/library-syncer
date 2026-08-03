package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/antoni-ostrowski/library-syncer/internal/downloader"
)

type Tracker struct {
	Id         string
	ReadRanges []ReadRange
	Artist     string
	Status     string
}
type ReadRange struct {
	Name    string
	Mapping TrackerMapping
}

func NewTracker(artist string, id string, status string, readRanges []ReadRange) Tracker {
	return Tracker{
		Artist:     artist,
		Id:         id,
		Status:     status,
		ReadRanges: readRanges,
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

func Parse(csvPath string, trackerArtist string, readRangeMapping TrackerMapping) ([]downloader.Downloadable, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	var headers []string
	for {
		row, err := r.Read()
		if err == io.EOF {
			return nil, fmt.Errorf("could not find CSV header")
		}
		if err != nil {
			return nil, err
		}

		if isHeader(row, readRangeMapping) {
			headers = row
			break
		}
	}

	headerIdx := map[string]int{}
	// map each column to int
	for i, h := range headers {
		headerIdx[normalizeColumn(h)] = i
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
			if i, ok := headerIdx[normalizeColumn(col)]; ok && i < len(row) {
				return row[i]
			}
			return ""
		}

		track := downloader.Track{
			Artist: trackerArtist,
			Name:   get(readRangeMapping.Name),
			Links:  get(readRangeMapping.Links),
			Era:    get(readRangeMapping.Era),
			Notes:  get(readRangeMapping.Notes),
		}

		track.Name = strings.Join(strings.Fields(track.Name), " ")

		for link := range strings.FieldsSeq(track.Links) {
			switch {
			case strings.Contains(link, "pillows.su"):
				a := &downloader.DownloadableTrack{Track: track, Url: createPillowcaseLink(link), Source: downloader.SourcePillowcase}
				downloadables = append(downloadables, a)
			case strings.Contains(link, "soundcloud.com"):
				a := &downloader.DownloadableTrack{Track: track, Url: link, Source: downloader.SourceSc}
				downloadables = append(downloadables, a)
			}

		}

	}

	return downloadables, nil
}

func isHeader(row []string, mapping TrackerMapping) bool {
	nameColumn := normalizeColumn(mapping.Name)
	linksColumn := normalizeColumn(mapping.Links)
	if nameColumn == "" || linksColumn == "" {
		return false
	}

	foundName := false
	foundLinks := false

	for _, column := range row {
		normalized := normalizeColumn(column)
		if normalized == nameColumn {
			foundName = true
		}
		if normalized == linksColumn {
			foundLinks = true
		}
	}

	return foundName && foundLinks
}

func normalizeColumn(column string) string {
	return strings.ToLower(strings.Join(strings.Fields(column), " "))
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
