CREATE TABLE IF NOT EXISTS watch_progress (
    animation_id UUID PRIMARY KEY REFERENCES animations(id) ON DELETE CASCADE,
    position_seconds DOUBLE PRECISION NOT NULL DEFAULT 0,
    duration_seconds DOUBLE PRECISION NOT NULL DEFAULT 0,
    completed BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS watch_progress_updated_at_idx ON watch_progress (updated_at DESC);
