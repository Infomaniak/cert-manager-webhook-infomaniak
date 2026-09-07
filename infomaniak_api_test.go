package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func TestGetRecordIDMatchesUnquotedTXTTarget(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/2/zones/example.com/records", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		fmt.Fprint(w, `{"result":"success","data":[{"id":12,"source":"_acme-challenge","type":"TXT","target":"\"challenge-value\"","ttl":300}]}`)
	})

	ik := newTestClient(t, mux)

	recordID, err := ik.getRecordID("example.com", "_acme-challenge", "challenge-value", "TXT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recordID == nil || *recordID != 12 {
		t.Errorf("expected record id 12, got: %v", recordID)
	}
}

func TestGetRecordIDNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/2/zones/example.com/records", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":"success","data":[]}`)
	})

	ik := newTestClient(t, mux)

	recordID, err := ik.getRecordID("example.com", "_acme-challenge", "challenge-value", "TXT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recordID != nil {
		t.Errorf("expected nil record id, got: %d", *recordID)
	}
}

func TestEnsureDNSRecordCreatesWhenMissing(t *testing.T) {
	var createdBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/2/zones/example.com/records", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"result":"success","data":[]}`)
		case http.MethodPost:
			createdBody, _ = io.ReadAll(r.Body)
			fmt.Fprint(w, `{"result":"success","data":{"id":42,"source":"_acme-challenge","type":"TXT","target":"challenge-value","ttl":300}}`)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	ik := newTestClient(t, mux)

	if err := ik.EnsureDNSRecord("example.com", "_acme-challenge", "challenge-value", "TXT", 300); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sent InfomaniakDNSRecord
	if err := json.Unmarshal(createdBody, &sent); err != nil {
		t.Fatalf("invalid JSON body: %v (%s)", err, createdBody)
	}
	if sent.Source != "_acme-challenge" || sent.Target != "challenge-value" || sent.Type != "TXT" || sent.TTL != 300 {
		t.Errorf("unexpected record sent: %+v", sent)
	}
}

func TestEnsureDNSRecordSkipsWhenExists(t *testing.T) {
	created := false
	mux := http.NewServeMux()
	mux.HandleFunc("/2/zones/example.com/records", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"result":"success","data":[{"id":12,"source":"_acme-challenge","type":"TXT","target":"\"challenge-value\"","ttl":300}]}`)
		case http.MethodPost:
			created = true
			fmt.Fprint(w, `{"result":"success","data":{}}`)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	ik := newTestClient(t, mux)

	if err := ik.EnsureDNSRecord("example.com", "_acme-challenge", "challenge-value", "TXT", 300); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected no POST when the record already exists")
	}
}

func TestRemoveDNSRecordDeletesByID(t *testing.T) {
	deleted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/2/zones/example.com/records", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":"success","data":[{"id":12,"source":"_acme-challenge","type":"TXT","target":"\"challenge-value\"","ttl":300}]}`)
	})
	mux.HandleFunc("/2/zones/example.com/records/12", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		deleted = true
		fmt.Fprint(w, `{"result":"success","data":true}`)
	})

	ik := newTestClient(t, mux)

	if err := ik.RemoveDNSRecord("example.com", "_acme-challenge", "challenge-value", "TXT"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("expected DELETE /2/zones/example.com/records/12 to be called")
	}
}

func TestRemoveDNSRecordSkipsWhenAbsent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/2/zones/example.com/records", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":"success","data":[]}`)
	})
	mux.HandleFunc("/2/zones/example.com/records/", func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected DELETE call")
		fmt.Fprint(w, `{"result":"success","data":true}`)
	})

	ik := newTestClient(t, mux)

	if err := ik.RemoveDNSRecord("example.com", "_acme-challenge", "challenge-value", "TXT"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
