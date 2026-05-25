package utils

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
)

const (
	JSONContentType = "application/json"
)

var (
	ContentTypeHeader = http.CanonicalHeaderKey("Content-Type")

	AppVersion = gitTag + "+" + os.Getenv("APP_NAME")
	gitTag     string // this variable is set by the CI/CD pipeline
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		slog.Error("failed to write json response", "error", err)
		http.Error(w, "error", http.StatusNoContent)
		return
	}

	SetJSONContentTypeHeader(w)
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// SetJSONContentTypeHeader sets Content-Type header as application/json
func SetJSONContentTypeHeader(w http.ResponseWriter) {
	w.Header().Set(ContentTypeHeader, JSONContentType)
}
