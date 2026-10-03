package model

// Leaf package for domain types. downloader, db, parser, runner and web all
// import this; it must not import any of them.

type DebugLogFunc func(format string, a ...any)

type Track struct {
	Id string
	Artist string
	Era    string
	Name   string
	Notes  string
	Link   string
}

func GetTrackId(link string) string {
	trackId := ""
	if len(link) >= 32 {
		trackId = link[len(link)-32:]
	}
	return trackId
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
