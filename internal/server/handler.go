package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Grandeath/order-service/internal/metrics"
	"github.com/Grandeath/order-service/internal/utils"
)

type TagVersion struct {
	Version string `json:"version"`
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	metrics.Paths.NotFound()
	w.WriteHeader(http.StatusNotFound)
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	pathname := strings.ReplaceAll(strings.Trim(r.URL.Path, "/"), "/", "_")
	metrics.Paths.MethodNotAllowed(fmt.Sprintf("%s - %s", r.Method, pathname))
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func ping(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("pong"))
}

func appVersion(w http.ResponseWriter, _ *http.Request) {
	utils.WriteJSON(w, http.StatusOK, TagVersion{Version: utils.AppVersion})
}
