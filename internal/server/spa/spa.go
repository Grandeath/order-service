package spa

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

type SPAHandler struct {
	fileServer http.Handler
	dist       fs.FS
	indexHTML  []byte
	indexInfo  fs.FileInfo
}

func NewSPAHandler(publicDir string) (*SPAHandler, error) {
	distFS := os.DirFS(publicDir)

	stat, err := fs.Stat(distFS, "index.html")
	if err != nil {
		return nil, fmt.Errorf("unable to stat %s/index.html: %w", publicDir, err)
	}

	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return nil, fmt.Errorf("unable to read %s/index.html: %w", publicDir, err)
	}

	return &SPAHandler{
		fileServer: http.FileServer(http.Dir(publicDir)),
		dist:       distFS,
		indexHTML:  indexHTML,
		indexInfo:  stat,
	}, nil
}

func (s *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		s.serveIndex(w, r)
		return
	}

	_, err := fs.Stat(s.dist, path)
	switch {
	case err == nil:
		s.fileServer.ServeHTTP(w, r)
		return
	case errors.Is(err, fs.ErrNotExist):
		s.serveIndex(w, r)
		return
	default:
		http.Error(w,
			fmt.Sprintf("unable to serve %s: %v", r.URL.Path, err),
			http.StatusInternalServerError)
		return
	}
}

func (s *SPAHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, s.indexInfo.Name(), s.indexInfo.ModTime(), bytes.NewReader(s.indexHTML))
}
