-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS animations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    video_path TEXT NOT NULL,
    poster_path TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'video/mp4',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS animations_name_idx ON animations (name);
CREATE INDEX IF NOT EXISTS animations_created_at_idx ON animations (created_at DESC);
