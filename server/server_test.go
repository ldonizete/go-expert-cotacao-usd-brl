package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeAPIBody = `{"USDBRL":{"code":"USD","codein":"BRL","bid":"5.4321","ask":"5.4400"}}`

func newTestServer(t *testing.T, apiHandler http.HandlerFunc) (*server, *sql.DB) {
	t.Helper()

	api := httptest.NewServer(apiHandler)
	t.Cleanup(api.Close)

	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := initDB(db); err != nil {
		t.Fatal(err)
	}

	return &server{db: db, apiURL: api.URL, apiTimeout: time.Second, dbTimeout: time.Second}, db
}

func countRows(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cotacoes`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestHandleCotacaoSuccess(t *testing.T) {
	s, db := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fakeAPIBody))
	})

	rec := httptest.NewRecorder()
	s.handleCotacao(rec, httptest.NewRequest(http.MethodGet, "/cotacao", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"bid":"5.4321"}` {
		t.Errorf("body = %s", got)
	}
	if n := countRows(t, db); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

func TestHandleCotacaoAPITimeout(t *testing.T) {
	s, db := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-r.Context().Done():
		}
		w.Write([]byte(fakeAPIBody))
	})
	s.apiTimeout = 50 * time.Millisecond

	rec := httptest.NewRecorder()
	s.handleCotacao(rec, httptest.NewRequest(http.MethodGet, "/cotacao", nil))

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", rec.Code)
	}
	if n := countRows(t, db); n != 0 {
		t.Errorf("rows = %d, want 0", n)
	}
}

func TestHandleCotacaoDBTimeoutStillReturnsQuote(t *testing.T) {
	s, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fakeAPIBody))
	})
	s.dbTimeout = time.Nanosecond

	rec := httptest.NewRecorder()
	s.handleCotacao(rec, httptest.NewRequest(http.MethodGet, "/cotacao", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"bid":"5.4321"}` {
		t.Errorf("body = %s", got)
	}
	// Não afirmamos nada sobre as linhas gravadas: o driver pode concluir o
	// INSERT antes de observar o cancelamento do contexto.
}

func TestHandleCotacaoAPIBadStatus(t *testing.T) {
	s, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	rec := httptest.NewRecorder()
	s.handleCotacao(rec, httptest.NewRequest(http.MethodGet, "/cotacao", nil))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}
