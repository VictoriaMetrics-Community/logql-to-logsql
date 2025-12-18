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
	if qi.LogsQL != `{app="nginx"} | stats by (_stream) rate() as value` {
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
	if qi.LogsQL != `{app="nginx"} | stats rate() as value` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}
