package youtrack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(serverURL string) *Client {
	return NewClient(serverURL, "test-token")
}

func TestCreateIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected 'POST' request, got '%s'", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&Issue{
			ID:      "new-issue",
			Summary: "New Issue",
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	dueDate := time.Now()
	issue, err := client.CreateIssue("project-id", "New Issue", "Description", &dueDate)
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	if issue.Summary != "New Issue" {
		t.Errorf("expected issue summary to be 'New Issue', got '%s'", issue.Summary)
	}
}

func TestUpdateIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected 'POST' request, got '%s'", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	dueDate := time.Now()
	err := client.UpdateIssue("issue-id", "Updated Issue", "Description", &dueDate)
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}
}

func TestGetIssueBySummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Issue{
			{ID: "found-issue", Summary: "Found Issue"},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	issue, err := client.GetIssueBySummary("project-id", "Found Issue")
	if err != nil {
		t.Fatalf("GetIssueBySummary() error = %v", err)
	}

	if issue.Summary != "Found Issue" {
		t.Errorf("expected issue summary to be 'Found Issue', got '%s'", issue.Summary)
	}
}

func TestGetUpdatedIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Issue{
			{ID: "updated-issue", Summary: "Updated Issue"},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	issues, err := client.GetUpdatedIssues("project-id", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("GetUpdatedIssues() error = %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Summary != "Updated Issue" {
		t.Errorf("expected issue summary to be 'Updated Issue', got '%s'", issues[0].Summary)
	}
}

func TestUpdateIssue_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	err := client.UpdateIssue("non-existent-issue", "Summary", "Description", nil)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestGetIssueBySummary_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "[]")
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	issue, err := client.GetIssueBySummary("project-id", "Non Existent")
	if err != nil {
		t.Fatalf("GetIssueBySummary() error = %v", err)
	}
	if issue != nil {
		t.Errorf("Expected no issue to be found, but got one: %+v", issue)
	}
}