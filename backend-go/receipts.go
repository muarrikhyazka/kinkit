package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ---------- receipt photos ----------
//
// One optional photo per spending row, stored on disk (not in Postgres):
//   $DATA_DIR/receipts/<account_id>/<random>.<ext>
// spending.receipt_path holds the path relative to $DATA_DIR/receipts.
// Every handler scopes by account_id, so another household can never read,
// replace or delete a receipt even if it guesses a spending id.

const maxReceiptBytes = 5 << 20 // 5 MB

// dataDir is where uploaded files live (docker-compose mounts ./data here).
func dataDir() string { return getenv("DATA_DIR", "/app/data") }

func receiptsRoot() string { return filepath.Join(dataDir(), "receipts") }

// sniffImage identifies an upload by its bytes (never the client's
// Content-Type / filename). Returns the served MIME type and file extension.
func sniffImage(head []byte) (mime, ext string, ok bool) {
	switch http.DetectContentType(head) {
	case "image/jpeg":
		return "image/jpeg", "jpg", true
	case "image/png":
		return "image/png", "png", true
	case "image/webp":
		return "image/webp", "webp", true
	}
	// HEIC/HEIF (iPhone camera): ISO-BMFF box "ftyp" + a HEIF brand.
	if len(head) >= 12 && string(head[4:8]) == "ftyp" {
		switch string(head[8:12]) {
		case "heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1":
			return "image/heic", "heic", true
		}
	}
	return "", "", false
}

var extMIME = map[string]string{"jpg": "image/jpeg", "png": "image/png", "webp": "image/webp", "heic": "image/heic"}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// receiptAbsPath resolves a stored relative path, refusing anything that
// would escape the receipts root.
func receiptAbsPath(rel string) (string, bool) {
	root := receiptsRoot()
	p := filepath.Join(root, filepath.Clean("/"+rel))
	if !strings.HasPrefix(p, root+string(os.PathSeparator)) {
		return "", false
	}
	return p, true
}

func removeReceiptFile(rel *string) {
	if rel == nil || *rel == "" {
		return
	}
	if p, ok := receiptAbsPath(*rel); ok {
		_ = os.Remove(p)
	}
}

// spendingReceipt returns the receipt path for a row owned by aid.
// found=false when the row doesn't exist for this account.
func (app *App) spendingReceipt(ctx context.Context, aid, sid int64) (path *string, found bool, err error) {
	err = app.DB.QueryRow(ctx, "SELECT receipt_path FROM spending WHERE id=$1 AND account_id=$2", sid, aid).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	return path, err == nil, err
}

// POST /api/spending/{sid}/receipt  (multipart field "file")
func (app *App) handleUploadReceipt(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	sid, err := urlInt64(r, "sid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := r.Context()
	old, found, err := app.spendingReceipt(ctx, aid, sid)
	if err != nil {
		dbError(w, err)
		return
	}
	if !found {
		jsonError(w, http.StatusNotFound, "spending not found")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxReceiptBytes+64<<10)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		jsonError(w, http.StatusRequestEntityTooLarge, "upload too large or malformed (max 5 MB)")
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	f, _, err := r.FormFile("file")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxReceiptBytes+1))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "could not read file")
		return
	}
	if len(data) > maxReceiptBytes {
		jsonError(w, http.StatusRequestEntityTooLarge, "receipt image too large (max 5 MB)")
		return
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	_, ext, ok := sniffImage(head)
	if !ok {
		jsonError(w, http.StatusUnsupportedMediaType, "unsupported image type (use JPEG, PNG, WebP or HEIC)")
		return
	}

	acctDir := strconv.FormatInt(aid, 10)
	rel := filepath.Join(acctDir, randomHex(16)+"."+ext)
	abs, _ := receiptAbsPath(rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		jsonError(w, http.StatusInternalServerError, "could not store receipt")
		return
	}
	if err := os.WriteFile(abs, data, 0o640); err != nil {
		jsonError(w, http.StatusInternalServerError, "could not store receipt")
		return
	}
	tag, err := app.DB.Exec(ctx, "UPDATE spending SET receipt_path=$1 WHERE id=$2 AND account_id=$3", rel, sid, aid)
	if err != nil || tag.RowsAffected() == 0 {
		_ = os.Remove(abs)
		if err != nil {
			dbError(w, err)
		} else {
			jsonError(w, http.StatusNotFound, "spending not found")
		}
		return
	}
	removeReceiptFile(old) // replaced
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "has_receipt": true})
}

// GET /api/spending/{sid}/receipt - streams the image (auth-checked).
func (app *App) handleGetReceipt(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	sid, err := urlInt64(r, "sid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	rel, found, err := app.spendingReceipt(r.Context(), aid, sid)
	if err != nil {
		dbError(w, err)
		return
	}
	if !found || rel == nil || *rel == "" {
		jsonError(w, http.StatusNotFound, "no receipt")
		return
	}
	abs, ok := receiptAbsPath(*rel)
	if !ok {
		jsonError(w, http.StatusNotFound, "no receipt")
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		jsonError(w, http.StatusNotFound, "receipt file missing")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		jsonError(w, http.StatusNotFound, "receipt file missing")
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

// DELETE /api/spending/{sid}/receipt
func (app *App) handleDeleteReceipt(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	sid, err := urlInt64(r, "sid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var old *string
	err = app.DB.QueryRow(r.Context(),
		"UPDATE spending SET receipt_path=NULL WHERE id=$1 AND account_id=$2 RETURNING (SELECT receipt_path FROM spending WHERE id=$1 AND account_id=$2)",
		sid, aid).Scan(&old)
	if errors.Is(err, pgx.ErrNoRows) {
		jsonError(w, http.StatusNotFound, "spending not found")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	removeReceiptFile(old)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "has_receipt": false})
}
