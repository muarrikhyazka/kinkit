package main

import (
	"context"
	"net/http"
	"time"
)

// assets

type assetRow struct {
	ID          int64   `json:"id"`
	AccountID   int64   `json:"account_id"`
	Description string  `json:"description"`
	Value       int64   `json:"value"`
	AccountRef  *int64  `json:"account_ref"`
	Kind        string  `json:"kind"`
	AccountName *string `json:"account_name"`
}

type assetTargetRow struct {
	ID          int64  `json:"id"`
	AccountID   int64  `json:"account_id"`
	TargetValue int64  `json:"target_value"`
	LastUpdate  string `json:"last_update"`
}

func (app *App) handleGetAssets(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	ctx := context.Background()
	rows, err := app.DB.Query(ctx,
		"SELECT a.id, a.account_id, a.description, a.value, a.account_ref, a.kind, ac.name FROM assets a "+
			"LEFT JOIN accounts ac ON ac.id=a.account_ref WHERE a.account_id=$1 ORDER BY a.id", aid)
	if err != nil {
		dbError(w, err)
		return
	}
	assets := []assetRow{}
	for rows.Next() {
		var a assetRow
		if err := rows.Scan(&a.ID, &a.AccountID, &a.Description, &a.Value, &a.AccountRef, &a.Kind, &a.AccountName); err != nil {
			rows.Close()
			dbError(w, err)
			return
		}
		assets = append(assets, a)
	}
	rows.Close()

	var target *assetTargetRow
	var t assetTargetRow
	var lastUpdate *string
	err = app.DB.QueryRow(ctx, "SELECT id, account_id, target_value, last_update FROM asset_target WHERE account_id=$1 ORDER BY id DESC LIMIT 1", aid).
		Scan(&t.ID, &t.AccountID, &t.TargetValue, &lastUpdate)
	if err == nil {
		if lastUpdate != nil {
			t.LastUpdate = *lastUpdate
		}
		target = &t
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": assets, "target": target})
}

func (app *App) handleAddAsset(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d struct {
		Description string `json:"description"`
		Value       int64  `json:"value"`
		AccountID   *int64 `json:"account_id"`
		Kind        string `json:"kind"`
	}
	_ = decodeJSON(r, &d)
	if d.Kind == "" {
		d.Kind = "liquid"
	}
	_, err := app.DB.Exec(context.Background(),
		"INSERT INTO assets (account_id, description, value, account_ref, kind) VALUES ($1,$2,$3,$4,$5)",
		aid, d.Description, d.Value, d.AccountID, d.Kind)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	assetID, err := urlInt64(r, "aid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d struct {
		Value int64 `json:"value"`
	}
	_ = decodeJSON(r, &d)
	if _, err := app.DB.Exec(context.Background(), "UPDATE assets SET value=$1 WHERE id=$2 AND account_id=$3", d.Value, assetID, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	assetID, err := urlInt64(r, "aid")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := app.DB.Exec(context.Background(), "DELETE FROM assets WHERE id=$1 AND account_id=$2", assetID, aid); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleSetAssetTarget(w http.ResponseWriter, r *http.Request) {
	aid, ok := app.requireAccount(w, r)
	if !ok {
		return
	}
	var d struct {
		TargetValue int64 `json:"target_value"`
	}
	_ = decodeJSON(r, &d)
	today := time.Now().UTC().Format("2006-01-02")
	_, err := app.DB.Exec(context.Background(),
		"INSERT INTO asset_target (account_id, target_value, last_update) VALUES ($1,$2,$3)", aid, d.TargetValue, today)
	if err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
