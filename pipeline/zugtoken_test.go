package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Jeder Bau eines Zugs holt ein frisches Token (13.11.2026: nach über einer Stunde Tests scheiterte der Klon der
// zweiten Hälfte eines roten Zugs mit dem Token vom Start).
func TestZugMitFrischemToken(t *testing.T) {
	var anfragen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anfragen++
		if r.URL.Path != "/pipeline/accepted" || r.Header.Get("x-signature") == "" {
			http.Error(w, "unexpected", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode([]einreichung{
			{EinreichungID: 9, Slug: "anderes", GitToken: "dummy-anderes"},
			{EinreichungID: 3, Slug: "spiel", GitToken: "dummy-neu"},
		})
	}))
	defer srv.Close()
	alt := plattform
	plattform = srv.URL
	defer func() { plattform = alt }()

	zug := []einreichung{{EinreichungID: 1, Slug: "spiel", GitToken: "dummy-alt"}, {EinreichungID: 2, Slug: "spiel", GitToken: "dummy-alt"}}
	neu := mitFrischemToken(zug)
	for _, e := range neu {
		if e.GitToken != "dummy-neu" {
			t.Fatalf("submission %d token %q, want the fresh one", e.EinreichungID, e.GitToken)
		}
	}
	if zug[0].GitToken != "dummy-alt" {
		t.Fatal("the caller's train was changed")
	}
	// Mit Deploy-Key (kein Token) fragt der Zug die Plattform nicht
	vorher := anfragen
	if r := mitFrischemToken([]einreichung{{EinreichungID: 1, Slug: "spiel"}}); r[0].GitToken != "" || anfragen != vorher {
		t.Fatal("deploy-key train asked the platform for a token")
	}
}
