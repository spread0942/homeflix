ALTER TABLE animations
    ADD COLUMN IF NOT EXISTS playback_status TEXT NOT NULL DEFAULT 'ready';

UPDATE animations
SET playback_status = 'ready'
WHERE playback_status IS NULL OR playback_status = '';
