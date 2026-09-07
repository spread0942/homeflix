# Homeflix

Personal Netflix-style film streaming. No authentication — local / home use only.

## Stack

- **Traefik** — reverse proxy on port 80 (`v3.6`, Docker API compatible with modern daemons)
- **Vue 3** frontend — library, series, search, player, admin upload
- **Go** API — Postgres metadata + file upload/stream
- **Postgres 16** — `series` + `animations` tables

## Quick start (hot reload)

```bash
docker compose up --build
```

Open [http://localhost](http://localhost).

The frontend runs **Vite** with hot module reload (via [`docker-compose.override.yml`](docker-compose.override.yml)): edit files under `frontend/` and the browser updates without rebuilding the image.

**Production-style nginx build** (no hot reload):

```bash
docker compose -f docker-compose.yml up --build
```

- **Home** — series cards + standalone films (search both)
- **Series** — seasons / parts list for a franchise or show
- **Watch** — HTML5 player with range seeking + keyboard shortcuts
- **Conversions (`/conversions`)** — live conversion queue (processing / ready / failed)
- **Admin (`/admin`)** — create series, upload films with season/episode/sort

## Grouping films

1. Create a **series** (e.g. Dune, Bleach) with kind `franchise`, `anime`, or `tv`.
2. Upload each film/episode and assign it to that series.
3. Use **sort / part** for movie trilogies (Part 1, Part 2).
4. Use **season + episode** for TV/anime.

## Environment (backend)

| Variable | Default (Compose) | Purpose |
|----------|-------------------|---------|
| `DATABASE_URL` | `postgres://sunny:sunny@postgres:5432/thousand_sunny?sslmode=disable` | Postgres connection (name kept for existing volume) |
| `MEDIA_ROOT` | `/data` | Video + poster storage |
| `PORT` | `8080` | API listen port |

Media files live in the Docker volume `media_data`. **Firefox/Chrome need H.264 video + AAC audio in an MP4** (not MKV/HEVC). Unsupported uploads are auto-converted with ffmpeg in the background (`playback_status`: `processing` → `ready`). Poster images are optional: upload one (stored as **WebP**) or leave empty and a preview frame is extracted from the video.

## Local frontend outside Compose (optional)

```bash
cd frontend && npm install && npm run dev
```

Vite proxies `/api` to `http://localhost:8080` (start the API separately).

## Release (tagged containers)

Push a semver tag `vx.x.x` (e.g. `v1.0.0`) to trigger [`.github/workflows/release.yml`](.github/workflows/release.yml). It builds and pushes:

- `ghcr.io/spread0942/homeflix-frontend:v1.0.0` (+ `latest`)
- `ghcr.io/spread0942/homeflix-backend:v1.0.0` (+ `latest`)

```bash
git tag v1.0.0
git push origin v1.0.0
```

Deploy a release without rebuilding locally:

```bash
export HOMEFLIX_VERSION=v1.0.0
docker compose -f docker-compose.yml pull
docker compose -f docker-compose.yml up -d
```

If the GHCR packages are private, log in first: `echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin`.

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/library?q=` | Home grid: series + standalone films |
| `GET` | `/api/series?q=` | List series |
| `GET` | `/api/series/{id}` | Series + ordered entries |
| `GET` | `/api/series/{id}/poster` | Series cover (or first entry) |
| `POST` | `/api/series` | Multipart: `name`, `description`, `kind`, optional `poster` |
| `PUT` | `/api/series/{id}` | Multipart: update fields; optional new `poster` |
| `DELETE` | `/api/series/{id}` | Delete series + all entries/files |
| `GET` | `/api/animations?q=` | List / search all films |
| `GET` | `/api/animations/{id}` | Metadata |
| `GET` | `/api/animations/{id}/poster` | Poster image |
| `GET` | `/api/animations/{id}/stream` | Video (byte-range) |
| `POST` | `/api/animations` | Multipart: `name`, `description`, `video`, optional `poster`, optional `series_id`, `season`, `episode`, `sort_order` |
| `PUT` | `/api/animations/{id}` | Multipart: update metadata; optional new `poster` (video unchanged) |
| `DELETE` | `/api/animations/{id}` | Delete row + files |
| `GET` | `/api/health` | Health check |

## Security note

There is no auth. Do not expose this stack to the public internet without Traefik basic-auth, a VPN, or similar in front of at least `/admin`.
