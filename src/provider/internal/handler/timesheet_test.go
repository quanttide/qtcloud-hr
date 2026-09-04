package handler

// 工时 API 测试。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quanttide/qtcloud-human/src/provider/internal/store"
)

func newTestServer() *httptest.Server {
	ts := store.NewTimesheetStore()
	th := NewTimesheetHandler(ts)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /timesheets", th.List)
	mux.HandleFunc("POST /timesheets", th.Create)
	mux.HandleFunc("GET /timesheets/{id}", th.Get)
	mux.HandleFunc("PUT /timesheets/{id}", th.Update)
	mux.HandleFunc("DELETE /timesheets/{id}", th.Delete)
	return httptest.NewServer(mux)
}

func TestTimesheetCRUD(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/timesheets", "application/json",
		bytes.NewBufferString(`{"userId":"u-1","date":"2026-08-12","hours":6.5,"project":"课程云","category":"研发"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// Update
	req, _ := http.NewRequest("PUT", ts.URL+"/timesheets/"+created.ID,
		bytes.NewBufferString(`{"userId":"u-1","date":"2026-08-12","hours":7,"project":"课程云","category":"研发"}`))
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Delete
	req, _ = http.NewRequest("DELETE", ts.URL+"/timesheets/"+created.ID, nil)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestTimesheetValidation(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, body := range []string{
		`{"date":"2026-08-12","hours":6}`,        // 缺 userId
		`{"userId":"u-1","hours":6}`,             // 缺 date
		`{"userId":"u-1","date":"2026-08-12","hours":0}`,  // hours 0
		`{"userId":"u-1","date":"2026-08-12","hours":25}`, // hours > 24
	} {
		resp, _ := http.Post(ts.URL+"/timesheets", "application/json", bytes.NewBufferString(body))
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: %d, want 400", body, resp.StatusCode)
		}
		resp.Body.Close()
	}
}
