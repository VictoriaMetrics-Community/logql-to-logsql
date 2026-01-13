package vlogs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics-Community/logql-to-logsql/lib/logsql"
)

type EndpointConfig struct {
	Endpoint    string
	BearerToken string
}

type RequestParams struct {
	EndpointConfig
	Start    string
	End      string
	ExecMode string
}

type API struct {
	ec     EndpointConfig
	limit  uint32
	client *http.Client
}

func NewVLogsAPI(ec EndpointConfig, limit uint32) *API {
	return &API{
		ec:    ec,
		limit: limit,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (a *API) SetHTTPClient(client *http.Client) {
	a.client = client
}

func (a *API) Execute(ctx context.Context, qi *logsql.QueryInfo, params RequestParams) ([]byte, error) {
	if a.ec.Endpoint != "" && params.Endpoint != "" && a.ec.Endpoint != params.Endpoint {
		return nil, &APIError{
			Code:    http.StatusBadRequest,
			Message: "endpoint can be set either in config or in request, not both",
		}
	}
	recParams := params
	if recParams.Endpoint == "" {
		recParams.Endpoint = a.ec.Endpoint
		recParams.BearerToken = a.ec.BearerToken
	}

	if recParams.Endpoint == "" || strings.EqualFold(recParams.ExecMode, "translate") {
		return nil, nil
	}

	switch qi.Kind {
	case logsql.QueryKindLogs:
		return a.QueryLogs(ctx, qi.LogsQL, recParams)
	case logsql.QueryKindStats:
		if strings.TrimSpace(recParams.Start) == "" && strings.TrimSpace(recParams.End) == "" {
			return a.QueryStats(ctx, qi.LogsQL, recParams)
		}
		return a.QueryStatsRange(ctx, qi.LogsQL, recParams)
	default:
		return nil, &APIError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("vlogs: unsupported query kind %q", qi.Kind),
		}
	}
}

func (a *API) QueryLogs(ctx context.Context, logsQL string, params RequestParams) ([]byte, error) {
	form := url.Values{}
	form.Set("query", logsQL)
	form.Set("limit", fmt.Sprintf("%d", a.limit))
	if params.Start != "" {
		form.Set("start", params.Start)
	}
	if params.End != "" {
		form.Set("end", params.End)
	}
	return a.doForm(ctx, params, "/select/logsql/query", form)
}

func (a *API) QueryStats(ctx context.Context, logsQL string, params RequestParams) ([]byte, error) {
	form := url.Values{}
	form.Set("query", logsQL)
	if params.End != "" {
		form.Set("time", params.End)
	}
	return a.doForm(ctx, params, "/select/logsql/stats_query", form)
}

func (a *API) QueryStatsRange(ctx context.Context, logsQL string, params RequestParams) ([]byte, error) {
	form := url.Values{}
	form.Set("query", logsQL)
	if params.Start != "" {
		form.Set("start", params.Start)
	}
	if params.End != "" {
		form.Set("end", params.End)
	}
	form.Set("step", "1h")
	return a.doForm(ctx, params, "/select/logsql/stats_query_range", form)
}

func (a *API) doForm(ctx context.Context, params RequestParams, path string, form url.Values) ([]byte, error) {
	if params.Endpoint == "" {
		return nil, &APIError{
			Code:    http.StatusBadRequest,
			Message: "endpoint is required for query execution",
		}
	}
	reqURL, err := url.Parse(params.Endpoint)
	if err != nil {
		return nil, &APIError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("invalid endpoint URL: %s", params.Endpoint),
			Err:     err,
		}
	}
	reqURL = reqURL.JoinPath(path)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, &APIError{
			Code:    http.StatusBadGateway,
			Message: "failed to create request",
			Err:     err,
		}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if params.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+params.BearerToken)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, &APIError{
			Code:    http.StatusBadGateway,
			Message: "failed to execute request",
			Err:     err,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &APIError{
			Code:    http.StatusBadGateway,
			Message: "failed to read response body",
			Err:     err,
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return nil, &APIError{
			Code:    http.StatusBadGateway,
			Message: fmt.Sprintf("status %d: %s", resp.StatusCode, msg),
		}
	}
	return body, nil
}
