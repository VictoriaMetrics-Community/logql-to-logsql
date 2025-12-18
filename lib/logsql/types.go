package logsql

type QueryKind string

const (
	QueryKindLogs  QueryKind = "logs"
	QueryKindStats QueryKind = "stats"
)

type QueryInfo struct {
	Kind   QueryKind
	LogsQL string
}
