package events

type Type string

const (
	SyncStarted  Type = "sync-started"
	SyncFinished Type = "sync-finished"
)

type Event struct {
	Type    Type
	Tracker string
}
