package main

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// File helpers for chore photos ($DATA_DIR/chores/<account>/<random>.<ext>),
// same approach as receipts.go (bytes sniffed, never the client's type).

// pathJoinClean joins rel under root, neutralising any "../" in rel.
func pathJoinClean(root, rel string) string {
	return filepath.Join(root, filepath.Clean("/"+rel))
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("too large")
	}
	return data, nil
}

func writeFileMkdir(abs string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return err
	}
	return os.WriteFile(abs, data, 0o640)
}

func removeFileQuiet(abs string) { _ = os.Remove(abs) }

// serveImageFile streams a stored image with a MIME type from its extension.
func serveImageFile(w http.ResponseWriter, r *http.Request, abs string) {
	f, err := os.Open(abs)
	if err != nil {
		jsonError(w, http.StatusNotFound, "photo file missing")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		jsonError(w, http.StatusNotFound, "photo file missing")
		return
	}
	mime := extMIME[strings.TrimPrefix(filepath.Ext(abs), ".")]
	if mime == "" {
		mime = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	http.ServeContent(w, r, filepath.Base(abs), st.ModTime(), f)
}
