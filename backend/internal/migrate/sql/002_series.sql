CREATE TABLE IF NOT EXISTS series (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    poster_path TEXT,
    kind TEXT NOT NULL DEFAULT 'franchise',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS series_name_idx ON series (name);
CREATE INDEX IF NOT EXISTS series_created_at_idx ON series (created_at DESC);

ALTER TABLE animations
    ADD COLUMN IF NOT EXISTS series_id UUID REFERENCES series(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS season INT,
    ADD COLUMN IF NOT EXISTS episode INT,
    ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS animations_series_id_idx ON animations (series_id);
CREATE INDEX IF NOT EXISTS animations_series_order_idx
    ON animations (series_id, season NULLS FIRST, episode NULLS FIRST, sort_order, created_at);
