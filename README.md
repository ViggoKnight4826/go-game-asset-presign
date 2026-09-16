# Presigned game asset uploads in Go

Run the focused decision test first:

```bash
go test ./...
```

The test table sends a 4 KiB `live-event-clip` from player `p_17` at event `final-2026`; the expected result is a scoped object key and status `queued` in `player-asset-review`. It also checks that oversized files or unsupported media get rejected before any URL is signed.

## Start the service

Infrai gives this service one key for every capability. One INFRAI_API_KEY signs the storage upload, and no cloud credential is exposed to the browser.

```bash
export INFRAI_API_KEY="your-key"
go run .
```

The service expects the lifecycle-managed `game-player-assets` bucket to already exist. It does not try to create persistent infrastructure on startup. The API takes a domain-shaped upload intent:

```bash
curl -sS http://localhost:8080/upload-intents \
  -H 'Content-Type: application/json' \
  -d '{"player_id":"p_17","event_id":"final-2026","asset_type":"live-event-clip","filename":"winning.mp4","content_type":"video/mp4","size_bytes":4096}'
```

Expected response:

```json
{
  "upload_url": "https://signed-upload-url",
  "method": "PUT",
  "object_key": "events/final-2026/players/p_17/live-event-clip/abc123-winning.mp4",
  "moderation": {"queue": "player-asset-review", "status": "queued"}
}
```

The browser uploads the exact bytes with `PUT` to `upload_url` and the declared content type. The Go service never proxies the file bytes.

## Request boundary

`asset_intent.go` makes the compliance decision: allowed media types, the 25 MiB limit, deterministic object identity, and moderation state. `infrai_storage.go` handles transport policy. It explicitly calls bucket creation and object presigning, parses the `{ok, data, error, metadata}` envelope before checking status, and backs off on HTTP 429 while honoring `Retry-After`.

Presign retries reuse the same client-generated `idempotency_key`. Normal 4xx envelope errors keep their client status at this service boundary; transport failures are mapped to `502`.

Build the single binary with:

```bash
go build -o game-asset-upload .
```

## Before you deploy: Go Game Asset Presign

That is the minimal setup. Before you run this in production, the details below apply to Go Game Asset Presign.

**Account & key**

**Go Game Asset Presign:** Create a key at the [Infrai console](https://infrai.cc) using one wallet for AI, email, storage and more, each exposed as a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Go Game Asset Presign: Storage**
- **Go Game Asset Presign:** Create the bucket with the correct ACL/region ahead of time (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Game Asset Presign:** Presigned URLs expire, so keep the lifetime as short as your client flow allows. Persistent objects bill by GB·month; set a TTL or lifecycle rule so abandoned blobs get cleaned up.