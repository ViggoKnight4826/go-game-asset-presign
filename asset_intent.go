package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	assetBucket  = "game-player-assets"
	maxAssetSize = int64(25 * 1024 * 1024)
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type UploadIntent struct {
	PlayerID    string `json:"player_id"`
	EventID     string `json:"event_id"`
	AssetType   string `json:"asset_type"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type ModerationTicket struct {
	Queue  string `json:"queue"`
	Status string `json:"status"`
}

type UploadGrant struct {
	UploadURL  string           `json:"upload_url"`
	Method     string           `json:"method"`
	ObjectKey  string           `json:"object_key"`
	Moderation ModerationTicket `json:"moderation"`
}

type assetDecision struct {
	ObjectKey      string
	IdempotencyKey string
	Queue          string
}

func decideUpload(in UploadIntent) (assetDecision, error) {
	if !safeID.MatchString(in.PlayerID) || !safeID.MatchString(in.EventID) {
		return assetDecision{}, errors.New("player_id and event_id must use letters, numbers, underscore, or dash")
	}
	if in.AssetType != "player-art" && in.AssetType != "live-event-clip" {
		return assetDecision{}, errors.New("asset_type must be player-art or live-event-clip")
	}
	if in.SizeBytes <= 0 || in.SizeBytes > maxAssetSize {
		return assetDecision{}, fmt.Errorf("size_bytes must be between 1 and %d", maxAssetSize)
	}
	if in.ContentType != "image/png" && in.ContentType != "image/jpeg" && in.ContentType != "video/mp4" {
		return assetDecision{}, errors.New("content_type must be image/png, image/jpeg, or video/mp4")
	}
	name := filepath.Base(strings.TrimSpace(in.Filename))
	if name == "." || name == "" {
		return assetDecision{}, errors.New("filename is required")
	}
	seed := strings.Join([]string{in.PlayerID, in.EventID, in.AssetType, name, fmt.Sprint(in.SizeBytes)}, "\x00")
	sum := sha256.Sum256([]byte(seed))
	id := hex.EncodeToString(sum[:12])
	return assetDecision{
		ObjectKey:      fmt.Sprintf("events/%s/players/%s/%s/%s-%s", in.EventID, in.PlayerID, in.AssetType, id, name),
		IdempotencyKey: "asset-intent-" + id,
		Queue:          "player-asset-review",
	}, nil
}
