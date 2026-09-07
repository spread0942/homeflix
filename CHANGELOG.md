# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Edit films and series from Admin (`PUT /api/animations/{id}`, `PUT /api/series/{id}`)
- Optional poster replacement on update; video file is left unchanged

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

[Unreleased]: https://github.com/spread0942/homeflix/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/spread0942/homeflix/releases/tag/v1.0.0
