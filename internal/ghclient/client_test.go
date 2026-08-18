package ghclient_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arobson/proji/internal/ghclient"
)

func newTestClient(t *testing.T, mux *http.ServeMux) *ghclient.Client {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := ghclient.NewClient(nil, "test-token")
	if err := client.SetBaseURL(server.URL + "/"); err != nil {
		t.Fatalf("SetBaseURL() error = %v", err)
	}
	client.Sleep = func(time.Duration) {}
	client.PollInterval = time.Millisecond
	client.PollTimeout = time.Second
	return client
}

func TestClient_CurrentUser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"login":"learner"}`)
	})
	client := newTestClient(t, mux)

	got, err := client.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}
	if got != "learner" {
		t.Errorf("CurrentUser() = %q, want %q", got, "learner")
	}
}

func TestClient_CurrentUser_InvalidToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"Bad credentials"}`)
	})
	client := newTestClient(t, mux)

	if _, err := client.CurrentUser(context.Background()); err == nil {
		t.Fatal("CurrentUser() error = nil, want an error for bad credentials")
	}
}

func TestClient_ForkRepo_PollsUntilReady(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/instructor/homework-1/forks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("forks endpoint got method %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{
			"name": "homework-1",
			"owner": {"login": "learner"},
			"html_url": "https://github.com/learner/homework-1",
			"clone_url": "https://github.com/learner/homework-1.git"
		}`)
	})

	getCalls := 0
	mux.HandleFunc("/repos/learner/homework-1", func(w http.ResponseWriter, _ *http.Request) {
		getCalls++
		if getCalls < 3 {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"Not Found"}`)
			return
		}
		fmt.Fprint(w, `{"name": "homework-1", "owner": {"login": "learner"}}`)
	})

	client := newTestClient(t, mux)
	result, err := client.ForkRepo(context.Background(), "instructor", "homework-1")
	if err != nil {
		t.Fatalf("ForkRepo() error = %v", err)
	}
	want := ghclient.RepoResult{
		Owner:    "learner",
		Repo:     "homework-1",
		HTMLURL:  "https://github.com/learner/homework-1",
		CloneURL: "https://github.com/learner/homework-1.git",
	}
	if result != want {
		t.Errorf("ForkRepo() = %+v, want %+v", result, want)
	}
	if getCalls < 3 {
		t.Errorf("Repositories.Get called %d times, want at least 3 (poll retried)", getCalls)
	}
}

func TestClient_ForkRepo_TimesOutIfNeverReady(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/instructor/homework-1/forks", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"name": "homework-1", "owner": {"login": "learner"}}`)
	})
	mux.HandleFunc("/repos/learner/homework-1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	})

	client := newTestClient(t, mux)
	client.PollTimeout = 10 * time.Millisecond

	if _, err := client.ForkRepo(context.Background(), "instructor", "homework-1"); err == nil {
		t.Fatal("ForkRepo() error = nil, want a timeout error")
	}
}

func TestClient_ForkRepo_NotFoundSourceRepo(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/instructor/does-not-exist/forks", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	})

	client := newTestClient(t, mux)
	if _, err := client.ForkRepo(context.Background(), "instructor", "does-not-exist"); err == nil {
		t.Fatal("ForkRepo() error = nil, want an error for a missing source repo")
	}
}

func TestClient_CreateRepo(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/repos", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("create endpoint got method %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"name":"my-project"`) {
			t.Errorf("create request body = %s, want it to include the repo name", body)
		}
		if !strings.Contains(string(body), `"private":false`) {
			t.Errorf("create request body = %s, want private:false", body)
		}
		fmt.Fprint(w, `{
			"name": "my-project",
			"owner": {"login": "learner"},
			"html_url": "https://github.com/learner/my-project",
			"clone_url": "https://github.com/learner/my-project.git"
		}`)
	})
	mux.HandleFunc("/repos/learner/my-project", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"name": "my-project", "owner": {"login": "learner"}}`)
	})

	client := newTestClient(t, mux)
	result, err := client.CreateRepo(context.Background(), "my-project", false)
	if err != nil {
		t.Fatalf("CreateRepo() error = %v", err)
	}
	want := ghclient.RepoResult{
		Owner:    "learner",
		Repo:     "my-project",
		HTMLURL:  "https://github.com/learner/my-project",
		CloneURL: "https://github.com/learner/my-project.git",
	}
	if result != want {
		t.Errorf("CreateRepo() = %+v, want %+v", result, want)
	}
}

func TestClient_CreateRepo_Failure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/repos", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"message":"name already exists on this account"}`)
	})

	client := newTestClient(t, mux)
	if _, err := client.CreateRepo(context.Background(), "taken", true); err == nil {
		t.Fatal("CreateRepo() error = nil, want an error when the name is already taken")
	}
}

func TestClient_AddSSHKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/keys", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("keys endpoint got method %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "ssh-ecdsa-fake-key") {
			t.Errorf("request body = %s, want it to include the public key", body)
		}
		fmt.Fprint(w, `{"id": 1, "title": "proji on my-laptop", "key": "ssh-ecdsa-fake-key"}`)
	})

	client := newTestClient(t, mux)
	if err := client.AddSSHKey(context.Background(), "proji on my-laptop", "ssh-ecdsa-fake-key"); err != nil {
		t.Fatalf("AddSSHKey() error = %v", err)
	}
}

func TestClient_AddSSHKey_AlreadyInUse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/keys", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"message":"key is already in use"}`)
	})

	client := newTestClient(t, mux)
	err := client.AddSSHKey(context.Background(), "title", "ssh-ecdsa-fake-key")
	if err == nil {
		t.Fatal("AddSSHKey() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "key is already in use") {
		t.Errorf("AddSSHKey() error = %q, want it to mention the key is already in use", err.Error())
	}
}
