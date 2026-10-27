package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchBid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"bid":"5.4321"}`))
	}))
	defer srv.Close()

	got, err := fetchBid(srv.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got != "5.4321" {
		t.Errorf("bid = %q, want 5.4321", got)
	}
}

func TestFetchBidTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	_, err := fetchBid(srv.URL, 50*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
}

func TestFetchBidBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusGatewayTimeout)
	}))
	defer srv.Close()

	if _, err := fetchBid(srv.URL, time.Second); err == nil {
		t.Fatal("expected error for non-200 status")
	}
}
