CREATE TABLE IF NOT EXISTS tracks (
    id TEXT NOT NULL,
    tracker_id TEXT NOT NULL,
    metadata JSON NOT NULL,
    PRIMARY KEY (id, tracker_id)
);

