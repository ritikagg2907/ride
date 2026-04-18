CREATE TABLE IF NOT EXISTS ratings (
    trip_id    UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    rater_id   UUID NOT NULL,
    rater_role VARCHAR(10) NOT NULL, -- rider | driver
    ratee_id   UUID NOT NULL,
    ratee_role VARCHAR(10) NOT NULL,
    score      INTEGER NOT NULL CHECK (score BETWEEN 1 AND 5),
    comment    TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (trip_id, rater_id)
);
