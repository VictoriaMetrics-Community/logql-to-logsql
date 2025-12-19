package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestHandleQueryLogsSuccess(t *testing.T) {
	srv, err := NewServer(Config{Endpoint: "http://victoria", Limit: 1000})
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	srv.setHTTPClient(&http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/select/logsql/query" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			if err := req.ParseForm(); err != nil {
				t.Fatalf("failed to parse form: %v", err)
			}
			if got := req.Form.Get("query"); got != `{app="nginx"} "error"` {
				t.Fatalf("unexpected query sent: %q", got)
			}
			if got := req.Form.Get("limit"); got != "1000" {
				t.Fatalf("unexpected limit sent: %q", got)
			}
			if got := req.Form.Get("start"); got != "1" {
				t.Fatalf("unexpected start sent: %q", got)
			}
			if got := req.Form.Get("end"); got != "2" {
				t.Fatalf("unexpected end sent: %q", got)
			}
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"status":"ok"}`)),
				Header:     make(http.Header),
			}
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}),
	})

	reqBody := map[string]string{
		"logql": `{app="nginx"} |= "error"`,
		"start": "1",
		"end":   "2",
	}
	buf, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logql-to-logsql", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var resp struct {
		LogsQL string `json:"logsql"`
		Data   string `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if resp.LogsQL != `{app="nginx"} "error"` {
		t.Fatalf("unexpected LogsQL: %s", resp.LogsQL)
	}
	if resp.Data != `{"status":"ok"}` {
		t.Fatalf("unexpected victoria payload: %s", resp.Data)
	}
}

func TestHandleQueryStatsRangeSuccess(t *testing.T) {
	srv, err := NewServer(Config{Endpoint: "http://victoria", Limit: 1000})
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	srv.setHTTPClient(&http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/select/logsql/stats_query_range" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			if err := req.ParseForm(); err != nil {
				t.Fatalf("failed to parse form: %v", err)
			}
			if got := req.Form.Get("query"); got != `{app="nginx"} _time:5m | stats by (_stream) rate() as value` {
				t.Fatalf("unexpected query sent: %q", got)
			}
			if got := req.Form.Get("start"); got != "1" {
				t.Fatalf("unexpected start sent: %q", got)
			}
			if got := req.Form.Get("end"); got != "2" {
				t.Fatalf("unexpected end sent: %q", got)
			}
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"status":"success"}`)),
				Header:     make(http.Header),
			}
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}),
	})

	reqBody := map[string]string{
		"logql": `rate({app="nginx"}[5m])`,
		"start": "1",
		"end":   "2",
	}
	buf, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logql-to-logsql", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var resp struct {
		LogsQL string `json:"logsql"`
		Data   string `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if resp.LogsQL != `{app="nginx"} _time:5m | stats by (_stream) rate() as value` {
		t.Fatalf("unexpected LogsQL: %s", resp.LogsQL)
	}
	if resp.Data != `{"status":"success"}` {
		t.Fatalf("unexpected victoria payload: %s", resp.Data)
	}
}

func TestHandleQueryTranslateOnlySkipsVictoria(t *testing.T) {
	srv, err := NewServer(Config{Endpoint: "http://victoria", Limit: 1000})
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	srv.setHTTPClient(&http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP call to VictoriaLogs: %s", req.URL.Path)
			return nil, nil
		}),
	})

	reqBody := map[string]string{
		"logql":    `{app="nginx"} |= "error"`,
		"execMode": "translate",
	}
	buf, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logql-to-logsql", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var resp struct {
		LogsQL string `json:"logsql"`
		Data   string `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if resp.LogsQL != `{app="nginx"} "error"` {
		t.Fatalf("unexpected LogsQL: %s", resp.LogsQL)
	}
	if resp.Data != "" {
		t.Fatalf("expected empty data, got: %q", resp.Data)
	}
}
