package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDecideUpload(t *testing.T) {
	tests := []struct {
		name      string
		input     UploadIntent
		wantQueue string
		wantErr   string
	}{
		{
			name:      "event clip enters review",
			input:     UploadIntent{PlayerID: "p_17", EventID: "final-2026", AssetType: "live-event-clip", Filename: "winning.mp4", ContentType: "video/mp4", SizeBytes: 4096},
			wantQueue: "player-asset-review",
		},
		{
			name:    "oversize asset is rejected before signing",
			input:   UploadIntent{PlayerID: "p_17", EventID: "final-2026", AssetType: "player-art", Filename: "banner.png", ContentType: "image/png", SizeBytes: maxAssetSize + 1},
			wantErr: "size_bytes",
		},
		{
			name:    "unsupported media is rejected",
			input:   UploadIntent{PlayerID: "p_17", EventID: "final-2026", AssetType: "player-art", Filename: "bundle.zip", ContentType: "application/zip", SizeBytes: 200},
			wantErr: "content_type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decideUpload(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want text %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Queue != tt.wantQueue || !strings.Contains(got.ObjectKey, "/live-event-clip/") {
				t.Fatalf("decision = %#v", got)
			}
		})
	}
}

func TestPresignPutRequestBoundary(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization header missing")
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": map[string]string{"url": "https://upload.example/signed"}, "metadata": map[string]any{}})
	}))
	defer server.Close()
	client := newInfraiClient("test-key")
	client.baseURL = server.URL
	url, err := client.presignPut(context.Background(), "game-player-assets", "events/e1/players/p1/player-art/id-card.png", "image/png", 512, "asset-intent-id")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := "/v1/storage/object/presign/" + strings.Join([]string{"game-player-assets", "events", "e1", "players", "p1", "player-art", "id-card.png"}, "/")
	if url != "https://upload.example/signed" || gotPath != wantPath {
		t.Fatalf("url = %q, path = %q", url, gotPath)
	}
	if gotBody["op"] != "put" || gotBody["expires_seconds"] != float64(600) || gotBody["max_bytes"] != float64(512) {
		t.Fatalf("body = %#v", gotBody)
	}
}

func TestCallRetriesRateLimitAfterEnvelopeDecode(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"ok":false,"error":{"code":"RATE_LIMITED","message":"retry later"}}`)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": map[string]string{"url": "https://upload.example/signed"}})
	}))
	defer server.Close()
	client := newInfraiClient("test-key")
	client.baseURL = server.URL
	client.sleep = func(context.Context, time.Duration) error { return nil }
	_, err := client.presignPut(context.Background(), "b", "k", "image/png", 1, "intent")
	if err != nil || attempts != 2 {
		t.Fatalf("err = %v, attempts = %d", err, attempts)
	}
}
