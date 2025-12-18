package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/VictoriaMetrics-Community/logql-to-logsql/cmd/logql-to-logsql/web"
	"github.com/VictoriaMetrics-Community/logql-to-logsql/lib/logsql"
	"github.com/VictoriaMetrics-Community/logql-to-logsql/lib/vlogs"
)

type Config struct {
	ListenAddr  string `json:"listenAddr"`
	Endpoint    string `json:"endpoint"`
	BearerToken string `json:"bearerToken"`
	Limit       uint32 `json:"limit"`
}

type Server struct {
	api *vlogs.API
	mux *http.ServeMux
}

func NewServer(cfg Config) (*Server, error) {
	serverCfg := cfg
	serverCfg.BearerToken = strings.TrimSpace(serverCfg.BearerToken)
	serverCfg.Endpoint = strings.TrimSpace(serverCfg.Endpoint)
	if serverCfg.Endpoint != "" {
		if _, err := url.Parse(serverCfg.Endpoint); err != nil {
			return nil, fmt.Errorf("invalid endpoint URL: %w", err)
		}
	}

	srv := &Server{
		mux: http.NewServeMux(),
		api: vlogs.NewVLogsAPI(
			vlogs.EndpointConfig{
				Endpoint:    serverCfg.Endpoint,
				BearerToken: serverCfg.BearerToken,
			},
			serverCfg.Limit,
		),
	}
	srv.mux.HandleFunc("/healthz", withSecurityHeaders(srv.handleHealth))
	srv.mux.HandleFunc("/api/v1/logql-to-logsql", withSecurityHeaders(srv.handleQuery))
	srv.mux.HandleFunc("/api/v1/config", withSecurityHeaders(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"endpoint": serverCfg.Endpoint, "limit": serverCfg.Limit})
	}))
	srv.mux.HandleFunc("/", withSecurityHeaders(srv.handleStatic))
	return srv, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) setHTTPClient(client *http.Client) {
	s.api.SetHTTPClient(client)
}

func withSecurityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next(w, r)
	}
}

type queryRequest struct {
	LogQL       string `json:"logql"`
	Endpoint    string `json:"endpoint,omitempty"`
	BearerToken string `json:"bearerToken,omitempty"`
	Start       string `json:"start,omitempty"`
	End         string `json:"end,omitempty"`
	ExecMode    string `json:"execMode,omitempty"`
}

type queryResponse struct {
	LogsQL string `json:"logsql"`
	Data   string `json:"data,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: failed to decode request: %v", err)
		writeJSON(w, http.StatusBadRequest, queryResponse{Error: "invalid request payload"})
		return
	}

	logqlText := strings.TrimSpace(req.LogQL)
	if logqlText == "" {
		writeJSON(w, http.StatusBadRequest, queryResponse{Error: "logql query is required"})
		return
	}
	execMode := strings.TrimSpace(strings.ToLower(req.ExecMode))
	if execMode != "" && execMode != "translate" && execMode != "query" {
		writeJSON(w, http.StatusBadRequest, queryResponse{Error: "invalid execMode: possible values are translate and query"})
		return
	}
	start := strings.TrimSpace(req.Start)
	end := strings.TrimSpace(req.End)

	qi, err := logsql.TranslateLogQLToLogsQL(logqlText)
	if err != nil {
		log.Printf("ERROR: query translation failed: %v", err)
		var ae *vlogs.APIError
		var te *logsql.TranslationError
		if errors.As(err, &ae) {
			writeJSON(w, ae.Code, queryResponse{Error: ae.Message})
		} else if errors.As(err, &te) {
			writeJSON(w, te.Code, queryResponse{Error: te.Message})
		} else {
			writeJSON(w, http.StatusInternalServerError, queryResponse{Error: "query translation failed"})
		}
		return
	}

	resp := queryResponse{LogsQL: qi.LogsQL}
	data, err := s.api.Execute(r.Context(), qi, vlogs.RequestParams{
		EndpointConfig: vlogs.EndpointConfig{
			Endpoint:    req.Endpoint,
			BearerToken: req.BearerToken,
		},
		Start:    start,
		End:      end,
		ExecMode: execMode,
	})
	if err != nil {
		log.Printf("ERROR: query execution failed: %v", err)
		var ae *vlogs.APIError
		if errors.As(err, &ae) {
			writeJSON(w, ae.Code, queryResponse{Error: ae.Message})
		} else {
			writeJSON(w, http.StatusBadGateway, queryResponse{Error: "query execution failed"})
		}
		return
	}
	resp.Data = string(data)
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("ERROR: failed to encode JSON response: %v", err)
	}
}

var (
	indexOnce  sync.Once
	indexBytes []byte
	indexErr   error
	uiFS       = web.DistFS()
)

func loadIndex() ([]byte, error) {
	indexOnce.Do(func() {
		indexBytes, indexErr = web.ReadFile("index.html")
	})
	return indexBytes, indexErr
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cleaned := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if cleaned == "" || cleaned == "index.html" {
		index, err := loadIndex()
		if err != nil {
			http.Error(w, "ui not available", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
		return
	}

	file, err := uiFS.Open(cleaned)
	if err != nil {
		serveIndexFallback(w, r)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		serveIndexFallback(w, r)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read asset", http.StatusInternalServerError)
		return
	}

	if ct := mime.TypeByExtension(path.Ext(cleaned)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeContent(w, r, cleaned, info.ModTime(), bytes.NewReader(data))
}

func serveIndexFallback(w http.ResponseWriter, r *http.Request) {
	index, err := loadIndex()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
