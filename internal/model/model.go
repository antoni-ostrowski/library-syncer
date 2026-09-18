package model

import "strings"

// Leaf package for domain types. downloader, db, parser, runner and web all
// import this; it must not import any of them.

type DebugLogFunc func(format string, a ...any)

type Track struct {
	Artist string
	Era    string
	Name   string
	Notes  string
	Links  string
}

type Source int

const (
	SourcePillowcase Source = iota
	SourceSc
)

type Downloadable interface {
	URL() string
	BaseName() string
	TrackPtr() *Track
	GetSource() Source
}

type DownloadableTrack struct {
	Track  Track
	Url    string
	Source Source
}

func (d *DownloadableTrack) URL() string {
	return d.Url
}

func (d *DownloadableTrack) TrackPtr() *Track {
	return &d.Track
}

func (d *DownloadableTrack) GetSource() Source {
	return d.Source
}

func (d *DownloadableTrack) BaseName() string {
	switch d.Source {
	case SourceSc:
		return d.Track.Name + GetTrackSlug(d.Url)
	default:
		return d.Track.Name + GetTrackId(d.Url)
	}
}

func GetTrackId(link string) string {
	trackId := ""
	if len(link) >= 32 {
		trackId = link[len(link)-32:]
	}
	return "---" + trackId
}

func GetTrackSlug(link string) string {
	const prefix = "soundcloud.com/"
	_, slug, found := strings.Cut(link, prefix)
	if !found {
		return ""
	}

	if q := strings.IndexAny(slug, "?#"); q != -1 {
		slug = slug[:q]
	}
	slug = strings.TrimSuffix(slug, "/")
	slug = strings.ReplaceAll(slug, "/", "-")

	return "---" + slug
}

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
