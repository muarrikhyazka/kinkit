package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// shared helpers

func (app *App) requireAccount(w http.ResponseWriter, r *http.Request) (int64, bool) {
	aid, ok := currentAccountID(r)
	if !ok {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return 0, false
	}
	return aid, true
}

func dbError(w http.ResponseWriter, err error) {
	jsonError(w, http.StatusInternalServerError, "db error: "+err.Error())
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func urlInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, name), 10, 64)
}

func queryIntDefault(r *http.Request, name string, def int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func queryIntPtr(r *http.Request, name string) *int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &n
}

// valueMaxCeiling ports get_value_max(): value_max=10jt|100jt caps results.
func valueMaxCeiling(r *http.Request) *int64 {
	switch r.URL.Query().Get("value_max") {
	case "10jt":
		c := int64(10_000_000)
		return &c
	case "100jt":
		c := int64(100_000_000)
		return &c
	}
	return nil
}

// appendCeiling appends " AND <alias> <= $N" to query/args when a value_max
// filter is active - ports value_filter(alias).
func appendCeiling(query string, args []any, r *http.Request, alias string) (string, []any) {
	ceiling := valueMaxCeiling(r)
	if ceiling == nil {
		return query, args
	}
	args = append(args, *ceiling)
	return query + fmt.Sprintf(" AND %s <= $%d", alias, len(args)), args
}

func asInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case int:
		return int64(t), true
	case json.Number:
		n, err := t.Int64()
		return n, err == nil
	}
	return 0, false
}

func requireLogin(app *App, h func(http.ResponseWriter, *http.Request, *App, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		aid, ok := currentAccountID(r)
		if !ok {
			jsonError(w, http.StatusUnauthorized, "login required")
			return
		}
		h(w, r, app, aid)
	}
}

func pathInt64(r *http.Request, key string) (int64, bool) {
	v, err := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	return v, err == nil
}
