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

func TestTranslateConditionalDropLabel(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`{app="nginx"} | drop foo=~"bar"`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindLogs {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} | format if (foo:~"bar") "" as foo` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateConditionalKeepLabel(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`{app="nginx"} | keep foo=~"bar"`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindLogs {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} | format if (foo:~"bar") "<foo>" as foo` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateJSONExpressionParser(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`{app="nginx"} | json duration="duration"`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindLogs {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} | unpack_json fields (duration)` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateJSONExpressionParserRename(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`{app="nginx"} | json latency="duration"`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindLogs {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} | unpack_json fields (duration) | format "<duration>" as latency | delete duration` {
		t.Fatalf("unexpected LogsQL: %q", qi.LogsQL)
	}
}

func TestTranslateLogfmtExpressionParser(t *testing.T) {
	qi, err := TranslateLogQLToLogsQL(`{app="nginx"} | logfmt duration="duration"`)
	if err != nil {
		t.Fatalf("TranslateLogQLToLogsQL error: %v", err)
	}
	if qi.Kind != QueryKindLogs {
		t.Fatalf("unexpected kind: %q", qi.Kind)
	}
	if qi.LogsQL != `{app="nginx"} | unpack_logfmt fields (duration)` {
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
