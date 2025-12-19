package logsql

import "testing"

func TestTranslateLogQuery(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`{app="nginx"} |= "error"`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindLogs {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} "error"` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateLogQueryWithoutStreamSelector(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`|= "error"`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindLogs {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{} "error"` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateLogQueryWithParserAndFilter(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`{app="nginx"} | json | trace_id="abcdef"`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindLogs {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} | unpack_json | filter trace_id:=abcdef` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateMetricRate(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`rate({app="nginx"}[5m])`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindStats {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} _time:5m | stats by (_stream) rate() as value` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateMetricSumRate(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`sum(rate({app="nginx"}[5m]))`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindStats {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} _time:5m | stats rate() as value` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateMetricCountOverTime(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`count_over_time({app="nginx"}[5m])`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindStats {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} _time:5m | stats by (_stream) count() as value` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateMetricSumCountOverTime(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`sum(count_over_time({app="nginx"}[5m]))`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindStats {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} _time:5m | stats count() as value` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateMetricSumRateByLabel(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`sum by (severity) (rate({app="nginx"}[5m]))`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindStats {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} _time:5m | stats by (severity) rate() as value` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateMetricTopKSumRate(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`topk(5, sum by (severity) (rate({app="nginx"}[5m])))`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindStats {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} _time:5m | stats by (severity) rate() as value | first 5 (value desc)` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateMetricRateWithOffset(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`rate({app="nginx"}[5m] offset 1h)`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindStats {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} _time:5m offset 1h | stats by (_stream) rate() as value` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateMetricAvgOverTimeUnwrap(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`avg_over_time({app="nginx"} | unwrap duration [5m])`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindStats {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} _time:5m | stats by (_stream) avg(duration) as value` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}
