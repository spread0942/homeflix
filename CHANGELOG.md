# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.0.0] - 2026-09-23

### Added

- Edit films and series from Admin (`PUT /api/animations/{id}`, `PUT /api/series/{id}`)
- Optional poster replacement on update; video file is left unchanged
- Optional poster on upload: if omitted, a WebP preview frame is extracted from the video
- Continue watching and watch-progress tracking (API, database schema, and home/watch UI)
- Viewed and completed status for films and series, shown in the library
- Season selection for series on Series and Watch pages
- Series episode navigation helpers (labeling, season grouping, next/previous)
- Batch upload in Admin, with series default season/episode/sort prefill
- Batch find-and-replace for titles (including regex) with preview before apply
- Auto-derived film title from the uploaded video file name
- Watch playback progress bar, time display, and seek-by-jump controls
- Keyboard shortcuts overlay for playback controls on the watch page
- Conversions panel and dedicated films/series sections in Admin routing

### Changed

- Visual redesign across App, Home, Series, Watch, Admin, and Conversions (colors, borders, radii, hover states)
- Typography switched from Fraunces to Nunito
- Compact topbar styling simplified for a cleaner header
- Watch episode list layout reworked for clearer season/episode browsing
- Admin navigation and section routing restructured for films, series, and conversions

## [1.0.0] - 2026-09-05

### Added

- Homeflix personal streaming stack (Vue frontend, Go API, Postgres, Traefik)
- Series and film library with search, watch page, and admin upload
- Background ffmpeg transcoding to browser-friendly H.264/AAC MP4
- WebP poster conversion on upload
- Conversions workshop UI (`/conversions`) with retry
- Local frontend hot-reload via `docker-compose.override.yml`
- Keyboard and overlay playback controls on the watch page
- Tagged image release workflow (`ghcr.io`) and Compose image pins

### Changed

- Project renamed from Thousand Sunny to Homeflix
- Color scheme and contrast updates across the UI

[Unreleased]: https://github.com/spread0942/homeflix/compare/v2.0.0...HEAD
[2.0.0]: https://github.com/spread0942/homeflix/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/spread0942/homeflix/releases/tag/v1.0.0
