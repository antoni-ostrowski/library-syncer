CREATE TABLE IF NOT EXISTS tracks (
    pillow_id TEXT NOT NULL,
    tracker_id TEXT NOT NULL,
    metadata JSON NOT NULL,
    PRIMARY KEY (pillow_id, tracker_id)
);

CREATE TABLE IF NOT EXISTS trackers (
    id TEXT NOT NULL PRIMARY KEY,
    read_ranges TEXT NOT NULL,
    artist TEXT NOT NULL,
    status TEXT NOT NULL
);


