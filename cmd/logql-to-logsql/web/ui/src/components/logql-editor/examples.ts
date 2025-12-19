export const DEFAULT_EXAMPLE_ID = "line_contains";

export const EXAMPLES = [
  {
    id: "basic_selector",
    title: "Stream selector",
    logql: `{collector="otel-collector"}`,
  },
  {
    id: "line_contains",
    title: "Line filter (contains)",
    logql: `{collector="otel-collector"} |= "POST"`,
  },
  {
    id: "line_not_contains",
    title: "Line filter (negative)",
    logql: `{collector="otel-collector"} != "GET"`,
  },
  {
    id: "line_regexp",
    title: "Line filter (regexp)",
    logql: `{collector="otel-collector"} |~ "GET|POST"`,
  },
  {
    id: "label_filter",
    title: "Label filter",
    logql: `{collector="otel-collector"} | products > 5`,
  },
  {
    id: "json_and_label",
    title: "JSON parse + filter",
    logql: `{collector="otel-collector"} | json | trace_id!=""`,
  },
  {
    id: "json_extract_field",
    title: "JSON extract field",
    logql: `{collector="otel-collector"} | json duration="duration"`,
  },
  {
    id: "json_extract_rename",
    title: "JSON extract + rename",
    logql: `{collector="otel-collector"} | json duration_ms="duration"`,
  },
  {
    id: "logfmt_and_label",
    title: "logfmt parse + filter",
    logql: `{collector="otel-collector"} | logfmt | products >= 10`,
  },
  {
    id: "logfmt_extract_field",
    title: "logfmt extract field",
    logql: `{collector="otel-collector"} | logfmt status="status"`,
  },
  {
    id: "logfmt_extract_rename",
    title: "logfmt extract + rename",
    logql: `{collector="otel-collector"} | logfmt status_code="status"`,
  },
  {
    id: "drop_labels",
    title: "Drop labels",
    logql: `{collector="otel-collector"} | drop span_id, trace_id`,
  },
  {
    id: "drop_labels_conditional",
    title: "Drop labels (conditional)",
    logql: `{collector="otel-collector"} | drop trace_id=~"^abc"`,
  },
  {
    id: "rate",
    title: "Rate (metric query)",
    logql: `rate({collector="otel-collector"}[5m])`,
  },
  {
    id: "count_over_time",
    title: "Count over time (metric query)",
    logql: `count_over_time({collector="otel-collector"}[5m])`,
  },
  {
    id: "sum_rate",
    title: "Sum(rate(...))",
    logql: `sum(rate({collector="otel-collector"}[5m]))`,
  },
  {
    id: "topk_rate_by_label",
    title: "topk(K, sum by (label) (rate(...)))",
    logql: `topk(5, sum by (severity) (rate({collector="otel-collector"}[5m])))`,
  },
];
