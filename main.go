package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

type uploadService struct{ storage *infraiClient }

func (s uploadService) serveUploadIntent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method must be POST"})
		return
	}
	var in UploadIntent
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return
	}
	decision, err := decideUpload(in)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	uploadURL, err := s.storage.presignPut(r.Context(), assetBucket, decision.ObjectKey, in.ContentType, in.SizeBytes, decision.IdempotencyKey)
	if err != nil {
		status := http.StatusBadGateway
		var apiErr *infraiError
		if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
			status = apiErr.HTTPStatus
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, UploadGrant{
		UploadURL:  uploadURL,
		Method:     http.MethodPut,
		ObjectKey:  decision.ObjectKey,
		Moderation: ModerationTicket{Queue: decision.Queue, Status: "queued"},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := newInfraiClient(key)
	mux := http.NewServeMux()
	mux.HandleFunc("/upload-intents", (uploadService{storage: client}).serveUploadIntent)
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("asset upload service listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
