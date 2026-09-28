CREATE TABLE IF NOT EXISTS animation_subtitles (
    id UUID PRIMARY KEY,
    animation_id UUID NOT NULL REFERENCES animations(id) ON DELETE CASCADE,
    language TEXT NOT NULL DEFAULT 'und',
    label TEXT NOT NULL DEFAULT 'Subtitles',
    file_path TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS animation_subtitles_animation_id_idx
    ON animation_subtitles (animation_id);

CREATE UNIQUE INDEX IF NOT EXISTS animation_subtitles_animation_lang_uidx
    ON animation_subtitles (animation_id, language);

-- Move legacy single subtitle_path rows into the new table.
INSERT INTO animation_subtitles (id, animation_id, language, label, file_path)
SELECT gen_random_uuid(), a.id, 'und', 'Subtitles', a.subtitle_path
FROM animations a
WHERE a.subtitle_path IS NOT NULL
  AND a.subtitle_path <> ''
  AND NOT EXISTS (
      SELECT 1 FROM animation_subtitles s WHERE s.animation_id = a.id
  );
