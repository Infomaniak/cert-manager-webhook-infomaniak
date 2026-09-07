package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient returns an InfomaniakAPI client pointing at a test server
func newTestClient(t *testing.T, handler http.Handler) *InfomaniakAPI {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &InfomaniakAPI{apiToken: "test-token", baseURL: srv.URL}
}

func TestGetZoneByNameDirectMatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/2/zones/example.com/exists", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("unexpected Authorization header: %q", got)
		}
		fmt.Fprint(w, `{"result":"success","data":true}`)
	})

	ik := newTestClient(t, mux)

	zone, err := ik.GetZoneByName("example.com.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if zone != "example.com" {
		t.Errorf("expected zone `example.com`, got `%s`", zone)
	}
}

func TestGetZoneByNameWalkUp(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/2/zones/api.example.com/exists", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":"success","data":false}`)
	})
	mux.HandleFunc("/2/zones/example.com/exists", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":"success","data":true}`)
	})

	ik := newTestClient(t, mux)

	zone, err := ik.GetZoneByName("api.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if zone != "example.com" {
		t.Errorf("expected zone `example.com`, got `%s`", zone)
	}
}

func TestGetZoneByNameNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":"success","data":false}`)
	})

	ik := newTestClient(t, mux)

	if _, err := ik.GetZoneByName("api.example.com"); !errors.Is(err, ErrZoneNotFound) {
		t.Errorf("expected ErrZoneNotFound, got: %v", err)
	}
}

func TestRequestErrorEnvelope(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/2/zones/example.com/exists", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"result":"error","error":{"code":"not_authorized","description":"bad token"}}`)
	})

	ik := newTestClient(t, mux)

	if _, err := ik.zoneExists("example.com"); err == nil {
		t.Error("expected an error when the API returns an error envelope")
	}
}

func TestZoneExistsMissingData(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/2/zones/example.com/exists", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":"success"}`)
	})

	ik := newTestClient(t, mux)

	if _, err := ik.zoneExists("example.com"); err == nil {
		t.Error("expected an error when the response has no data")
	}
}
