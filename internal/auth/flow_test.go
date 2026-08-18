package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"golang.org/x/oauth2"

	"github.com/arobson/proji/internal/auth"
	"github.com/arobson/proji/internal/config"
	"github.com/arobson/proji/internal/prompt/prompttest"
)

func TestFlow_EnsureToken_ReturnsStoredTokenWhenStillValid(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "creds.yml"))
	if err := store.Save(config.Creds{Token: "stored-token", TokenType: "pat"}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	validateCalls := 0
	f := &auth.Flow{
		Store: store,
		Validate: func(_ context.Context, token string) (string, error) {
			validateCalls++
			if token != "stored-token" {
				t.Errorf("Validate called with %q, want %q", token, "stored-token")
			}
			return "learner", nil
		},
		Prompt: &prompttest.Fake{},
	}

	got, err := f.EnsureToken(context.Background())
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if got != "stored-token" {
		t.Errorf("EnsureToken() = %q, want %q", got, "stored-token")
	}
	if validateCalls != 1 {
		t.Errorf("Validate called %d times, want 1", validateCalls)
	}
}

func TestFlow_EnsureToken_PATFallbackWhenNoClientID(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "creds.yml"))
	fakePrompt := &prompttest.Fake{Answers: []string{"ghp_pastedtoken"}}

	f := &auth.Flow{
		Store: store,
		Validate: func(_ context.Context, token string) (string, error) {
			if token != "ghp_pastedtoken" {
				return "", fmt.Errorf("unexpected token %q", token)
			}
			return "learner", nil
		},
		Prompt: fakePrompt,
	}

	got, err := f.EnsureToken(context.Background())
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if got != "ghp_pastedtoken" {
		t.Errorf("EnsureToken() = %q, want %q", got, "ghp_pastedtoken")
	}

	saved, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after EnsureToken error = %v", err)
	}
	if saved.Token != "ghp_pastedtoken" || saved.TokenType != "pat" || saved.Username != "learner" {
		t.Errorf("saved creds = %+v, want token/type/username populated", saved)
	}
}

func TestFlow_EnsureToken_PATFallbackRetriesOnInvalidToken(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "creds.yml"))
	fakePrompt := &prompttest.Fake{Answers: []string{"bad-token", "good-token"}}

	f := &auth.Flow{
		Store: store,
		Validate: func(_ context.Context, token string) (string, error) {
			if token == "good-token" {
				return "learner", nil
			}
			return "", errors.New("bad credentials")
		},
		Prompt: fakePrompt,
	}

	got, err := f.EnsureToken(context.Background())
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if got != "good-token" {
		t.Errorf("EnsureToken() = %q, want %q", got, "good-token")
	}
}

func TestFlow_EnsureToken_DeviceFlow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login/device/code", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("client_id") != "test-client-id" {
			t.Errorf("device code request client_id = %q, want %q", r.FormValue("client_id"), "test-client-id")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "devcode123",
			"user_code":        "USER-CODE",
			"verification_uri": "https://github.com/login/device",
			"interval":         1,
			"expires_in":       60,
		})
	})

	pollCount := 0
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("device_code") != "devcode123" {
			t.Errorf("token request device_code = %q, want %q", r.FormValue("device_code"), "devcode123")
		}
		pollCount++
		w.Header().Set("Content-Type", "application/json")
		if pollCount < 2 {
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "authorization_pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "gho_realtoken",
			"token_type":   "bearer",
		})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	store := config.NewStore(filepath.Join(t.TempDir(), "creds.yml"))
	f := &auth.Flow{
		ClientID: "test-client-id",
		Store:    store,
		Prompt:   &prompttest.Fake{},
		Validate: func(_ context.Context, token string) (string, error) {
			if token != "gho_realtoken" {
				return "", fmt.Errorf("unexpected token %q", token)
			}
			return "learner", nil
		},
		Endpoint: oauth2.Endpoint{
			DeviceAuthURL: server.URL + "/login/device/code",
			TokenURL:      server.URL + "/login/oauth/access_token",
		},
	}

	got, err := f.EnsureToken(context.Background())
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if got != "gho_realtoken" {
		t.Errorf("EnsureToken() = %q, want %q", got, "gho_realtoken")
	}
	if pollCount < 2 {
		t.Errorf("token endpoint polled %d times, want at least 2 (pending then success)", pollCount)
	}

	saved, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after EnsureToken error = %v", err)
	}
	if saved.TokenType != "oauth" {
		t.Errorf("saved TokenType = %q, want %q", saved.TokenType, "oauth")
	}
}
