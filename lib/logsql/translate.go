package logsql

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	lokilog "github.com/grafana/loki/v3/pkg/logql/log"
	"github.com/grafana/loki/v3/pkg/logql/syntax"
	prommodel "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

func TranslateLogQLToLogsQL(query string) (*QueryInfo, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, &TranslationError{Code: http.StatusBadRequest, Message: "logql query is required"}
	}
	// In Loki, a log query must start with a stream selector (`{...}`).
	// Users often omit it (because in LogsQL it is optional) and start with a pipeline stage.
	// Be permissive and auto-prepend an empty selector.
	if strings.HasPrefix(q, "|") {
		q = "{} " + q
	}

	expr, err := syntax.ParseExpr(q)
	if err != nil {
		// Fall back to parsing without Loki validations. This allows translating
		// queries such as `|= "foo"` (missing selector), which are invalid in Loki
		// but can be mapped to LogsQL.
		expr, err = syntax.ParseExprWithoutValidation(q)
		if err != nil {
			return nil, newBadRequest("failed to parse LogQL", err)
		}
	}

	if se, ok := expr.(syntax.SampleExpr); ok {
		logsQL, err := translateSampleExpr(se)
		if err != nil {
			return nil, err
		}
		return &QueryInfo{Kind: QueryKindStats, LogsQL: logsQL}, nil
	}
	if le, ok := expr.(syntax.LogSelectorExpr); ok {
		b := newLogsQLBuilder()
		if err := b.addLogSelector(le); err != nil {
			return nil, err
		}
		return &QueryInfo{Kind: QueryKindLogs, LogsQL: b.String()}, nil
	}

	return nil, &TranslationError{
		Code:    http.StatusBadRequest,
		Message: fmt.Sprintf("unsupported LogQL expression type %T", expr),
	}
}

type logsQLBuilder struct {
	sb      strings.Builder
	hasPipe bool
}

func newLogsQLBuilder() *logsQLBuilder {
	return &logsQLBuilder{}
}

func (b *logsQLBuilder) String() string {
	return strings.TrimSpace(b.sb.String())
}

func (b *logsQLBuilder) addPipe(pipe string) {
	b.hasPipe = true
	b.sb.WriteString(" | ")
	b.sb.WriteString(pipe)
}

func (b *logsQLBuilder) addFilter(filter string) {
	f := strings.TrimSpace(filter)
	if f == "" {
		return
	}
	if b.hasPipe {
		b.addPipe("filter " + f)
		return
	}
	b.sb.WriteString(" ")
	b.sb.WriteString(f)
}

func (b *logsQLBuilder) addLogSelector(expr syntax.LogSelectorExpr) error {
	return b.addLogSelectorWithFilters(expr, nil)
}

func (b *logsQLBuilder) addLogSelectorWithFilters(expr syntax.LogSelectorExpr, filters []string) error {
	switch e := expr.(type) {
	case *syntax.MatchersExpr:
		b.sb.WriteString(renderStreamSelector(e.Matchers()))
		for _, f := range filters {
			b.addFilter(f)
		}
		return nil
	case *syntax.PipelineExpr:
		b.sb.WriteString(renderStreamSelector(e.Matchers()))
		for _, f := range filters {
			b.addFilter(f)
		}
		for _, stage := range e.MultiStages {
			if err := b.addStage(stage); err != nil {
				return err
			}
		}
		return nil
	default:
		return &TranslationError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported LogQL log selector type %T", expr),
		}
	}
}

func (b *logsQLBuilder) addStage(stage syntax.StageExpr) error {
	switch s := stage.(type) {
	case *syntax.LineFilterExpr:
		filters, err := translateLineFilterChain(s)
		if err != nil {
			return err
		}
		for _, f := range filters {
			b.addFilter(f)
		}
		return nil
	case *syntax.LabelFilterExpr:
		f, err := translateLabelFilterer(s.LabelFilterer)
		if err != nil {
			return err
		}
		b.addFilter(f)
		return nil
	case *syntax.LineParserExpr:
		pipe, err := translateLineParserPipe(s)
		if err != nil {
			return err
		}
		b.addPipe(pipe)
		return nil
	case *syntax.LogfmtParserExpr:
		b.addPipe("unpack_logfmt")
		return nil
	case *syntax.DecolorizeExpr:
		b.addPipe("decolorize")
		return nil
	case *syntax.DropLabelsExpr:
		raw := strings.TrimSpace(strings.TrimPrefix(s.String(), syntax.OpPipe+" "+syntax.OpDrop))
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil
		}
		parts := splitCommaOutsideQuotes(raw)
		if len(parts) == 0 {
			return nil
		}
		var pendingNames []string
		flushNames := func() {
			if len(pendingNames) == 0 {
				return
			}
			b.addPipe("delete " + strings.Join(pendingNames, ", "))
			pendingNames = nil
		}
		for _, part := range parts {
			item := strings.TrimSpace(part)
			if item == "" {
				continue
			}
			if strings.ContainsAny(item, "=!~") {
				flushNames()
				matcher, err := parseLabelMatcher(item)
				if err != nil {
					return newBadRequest("failed to parse LogQL drop label matcher", err)
				}
				cond, err := translateLabelsMatcher(matcher)
				if err != nil {
					return err
				}
				b.addPipe("format if (" + cond + ") \"\" as " + quoteFieldNameIfNeeded(matcher.Name))
				continue
			}
			pendingNames = append(pendingNames, item)
		}
		flushNames()
		return nil
	case *syntax.KeepLabelsExpr:
		// KeepLabelsExpr doesn't expose parsed items, so parse the string form.
		raw := strings.TrimSpace(strings.TrimPrefix(s.String(), syntax.OpPipe+" "+syntax.OpKeep))
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil
		}
		parts := splitCommaOutsideQuotes(raw)
		if len(parts) == 0 {
			return nil
		}
		var unconditional []string
		conditional := make([]*labels.Matcher, 0, len(parts))
		nameSeen := make(map[string]struct{})
		for _, part := range parts {
			item := strings.TrimSpace(part)
			if item == "" {
				continue
			}
			if strings.ContainsAny(item, "=!~") {
				matcher, err := parseLabelMatcher(item)
				if err != nil {
					return newBadRequest("failed to parse LogQL keep label matcher", err)
				}
				conditional = append(conditional, matcher)
				continue
			}
			if strings.ContainsAny(item, "\"`") {
				return &TranslationError{
					Code:    http.StatusBadRequest,
					Message: "invalid LogQL keep label; convert it manually (see logsql/logql-to-logsql.md)",
				}
			}
			unconditional = append(unconditional, item)
		}
		if len(unconditional) > 0 {
			keepNames := make([]string, 0, len(unconditional)+len(conditional))
			for _, name := range unconditional {
				if _, ok := nameSeen[name]; ok {
					continue
				}
				nameSeen[name] = struct{}{}
				keepNames = append(keepNames, name)
			}
			for _, matcher := range conditional {
				name := matcher.Name
				if _, ok := nameSeen[name]; ok {
					continue
				}
				nameSeen[name] = struct{}{}
				keepNames = append(keepNames, name)
			}
			if len(keepNames) > 0 {
				b.addPipe("keep " + strings.Join(keepNames, ", "))
			}
		}
		for _, matcher := range conditional {
			cond, err := translateLabelsMatcher(matcher)
			if err != nil {
				return err
			}
			pattern := "<" + matcher.Name + ">"
			b.addPipe("format if (" + cond + ") " + quoteString(pattern) + " as " + quoteFieldNameIfNeeded(matcher.Name))
		}
		return nil
	case *syntax.LineFmtExpr:
		b.addPipe("format " + quoteString(convertLokiTemplateToLogsQLPattern(s.Value)))
		return nil
	case *syntax.LabelFmtExpr:
		var renames []string
		for _, f := range s.Formats {
			if f.Rename {
				renames = append(renames, fmt.Sprintf("%s as %s", quoteFieldNameIfNeeded(f.Value), quoteFieldNameIfNeeded(f.Name)))
				continue
			}
			pattern := convertLokiTemplateToLogsQLPattern(f.Value)
			b.addPipe("format " + quoteString(pattern) + " as " + quoteFieldNameIfNeeded(f.Name))
		}
		if len(renames) > 0 {
			b.addPipe("rename " + strings.Join(renames, ", "))
		}
		return nil
	case *syntax.JSONExpressionParserExpr:
		pipes, err := translateLabelExtractionParser("unpack_json", s.Expressions)
		if err != nil {
			return err
		}
		for _, pipe := range pipes {
			b.addPipe(pipe)
		}
		return nil
	case *syntax.LogfmtExpressionParserExpr:
		pipes, err := translateLabelExtractionParser("unpack_logfmt", s.Expressions)
		if err != nil {
			return err
		}
		for _, pipe := range pipes {
			b.addPipe(pipe)
		}
		return nil
	default:
		return &TranslationError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported LogQL pipeline stage %T", stage),
		}
	}
}

func renderStreamSelector(matchers []*labels.Matcher) string {
	if len(matchers) == 0 {
		return "{}"
	}
	var sb strings.Builder
	sb.WriteString("{")
	for i, m := range matchers {
		sb.WriteString(m.String())
		if i+1 != len(matchers) {
			sb.WriteString(",")
		}
	}
	sb.WriteString("}")
	return sb.String()
}

func translateLineParserPipe(e *syntax.LineParserExpr) (string, error) {
	switch e.Op {
	case syntax.OpParserTypeJSON, syntax.OpParserTypeUnpack:
		return "unpack_json", nil
	case syntax.OpParserTypeLogfmt:
		return "unpack_logfmt", nil
	case syntax.OpParserTypeRegexp:
		return "extract_regexp " + quoteString(e.Param), nil
	case syntax.OpParserTypePattern:
		return "extract " + quoteString(e.Param), nil
	default:
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported LogQL parser stage %q", e.Op),
		}
	}
}

func translateLineFilterChain(e *syntax.LineFilterExpr) ([]string, error) {
	var out []string
	for curr := e; curr != nil; curr = curr.Left {
		if curr.IsOrChild {
			continue
		}
		s, err := translateLineFilterOrGroup(curr)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	// The chain is built right-to-left; restore original order.
	for i := 0; i < len(out)/2; i++ {
		out[i], out[len(out)-1-i] = out[len(out)-1-i], out[i]
	}
	return out, nil
}

func translateLineFilterOrGroup(e *syntax.LineFilterExpr) (string, error) {
	if e.Op != "" {
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported LogQL line filter function %q", e.Op),
		}
	}

	leaf, err := translateLineFilterLeaf(e.Ty, e.Match)
	if err != nil {
		return "", err
	}
	if e.Or == nil {
		return leaf, nil
	}

	if e.Ty != lokilog.LineMatchEqual && e.Ty != lokilog.LineMatchRegexp && e.Ty != lokilog.LineMatchPattern {
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: "LogQL line filter 'or' for negative matches isn't supported yet; rewrite the query without 'or'",
		}
	}

	parts := []string{leaf}
	for orNode := e.Or; orNode != nil; orNode = orNode.Or {
		p, err := translateLineFilterLeaf(orNode.Ty, orNode.Match)
		if err != nil {
			return "", err
		}
		parts = append(parts, p)
	}
	return "(" + strings.Join(parts, " OR ") + ")", nil
}

func translateLineFilterLeaf(ty lokilog.LineMatchType, match string) (string, error) {
	switch ty {
	case lokilog.LineMatchEqual:
		return quoteString(match), nil
	case lokilog.LineMatchNotEqual:
		return "-" + quoteString(match), nil
	case lokilog.LineMatchRegexp:
		return "~" + quoteString(match), nil
	case lokilog.LineMatchNotRegexp:
		return "NOT ~" + quoteString(match), nil
	case lokilog.LineMatchPattern, lokilog.LineMatchNotPattern:
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: "LogQL pattern line filters (|> / !>) aren't supported yet",
		}
	default:
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported LogQL line filter type %v", ty),
		}
	}
}

func translateLabelExtractionParser(pipe string, exprs []lokilog.LabelExtractionExpr) ([]string, error) {
	if len(exprs) == 0 {
		return []string{pipe}, nil
	}
	exprOrder := make([]string, 0, len(exprs))
	exprSeen := make(map[string]struct{}, len(exprs))
	exprToIDs := make(map[string][]string, len(exprs))
	keepExpr := make(map[string]bool, len(exprs))
	for _, exp := range exprs {
		expr := exp.Expression
		if expr == "" {
			return nil, &TranslationError{
				Code:    http.StatusBadRequest,
				Message: "empty json/logfmt extraction expression isn't supported; convert it manually (see logsql/logql-to-logsql.md)",
			}
		}
		if !isSimpleExtractionField(expr) {
			return nil, &TranslationError{
				Code:    http.StatusBadRequest,
				Message: "complex json/logfmt extraction expressions aren't supported yet; convert it manually (see logsql/logql-to-logsql.md)",
			}
		}
		if _, ok := exprSeen[expr]; !ok {
			exprSeen[expr] = struct{}{}
			exprOrder = append(exprOrder, expr)
		}
		exprToIDs[expr] = append(exprToIDs[expr], exp.Identifier)
		if exp.Identifier == expr {
			keepExpr[expr] = true
		}
	}
	fields := make([]string, 0, len(exprOrder))
	for _, expr := range exprOrder {
		fields = append(fields, quoteFieldNameIfNeeded(expr))
	}
	var pipes []string
	pipeStr := pipe
	if len(fields) > 0 {
		pipeStr += " fields (" + strings.Join(fields, ", ") + ")"
	}
	pipes = append(pipes, pipeStr)
	seenFormats := make(map[string]struct{}, len(exprs))
	for _, expr := range exprOrder {
		for _, id := range exprToIDs[expr] {
			if id == expr {
				continue
			}
			key := expr + "\x00" + id
			if _, ok := seenFormats[key]; ok {
				continue
			}
			seenFormats[key] = struct{}{}
			pattern := "<" + expr + ">"
			pipes = append(pipes, "format "+quoteString(pattern)+" as "+quoteFieldNameIfNeeded(id))
		}
	}
	var drop []string
	for _, expr := range exprOrder {
		if keepExpr[expr] {
			continue
		}
		drop = append(drop, quoteFieldNameIfNeeded(expr))
	}
	if len(drop) > 0 {
		pipes = append(pipes, "delete "+strings.Join(drop, ", "))
	}
	return pipes, nil
}

func parseLabelMatcher(raw string) (*labels.Matcher, error) {
	matchers, err := syntax.ParseMatchers("{"+raw+"}", false)
	if err != nil {
		return nil, err
	}
	if len(matchers) != 1 {
		return nil, fmt.Errorf("expected 1 matcher; got %d", len(matchers))
	}
	return matchers[0], nil
}

func splitCommaOutsideQuotes(s string) []string {
	var parts []string
	start := 0
	inQuotes := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if inQuotes && c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inQuotes = !inQuotes
			continue
		}
		if c == ',' && !inQuotes {
			part := strings.TrimSpace(s[start:i])
			if part != "" {
				parts = append(parts, part)
			}
			start = i + 1
		}
	}
	if start <= len(s) {
		part := strings.TrimSpace(s[start:])
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func translateLabelFilterer(f lokilog.LabelFilterer) (string, error) {
	switch t := f.(type) {
	case *lokilog.NoopLabelFilter:
		return "", nil
	case *lokilog.BinaryLabelFilter:
		left, err := translateLabelFilterer(t.Left)
		if err != nil {
			return "", err
		}
		right, err := translateLabelFilterer(t.Right)
		if err != nil {
			return "", err
		}
		op := " OR "
		if t.And {
			op = " AND "
		}
		return "(" + left + op + right + ")", nil
	case *lokilog.NumericLabelFilter:
		return translateScalarFilter(t.Name, t.Type, formatFloat(t.Value))
	case *lokilog.DurationLabelFilter:
		return translateScalarFilter(t.Name, t.Type, t.Value.String())
	case *lokilog.BytesLabelFilter:
		return translateScalarFilter(t.Name, t.Type, strconv.FormatUint(t.Value, 10))
	case *lokilog.IPLabelFilter:
		return translateIPFilter(t.Label, t.Ty, t.Pattern)
	case *lokilog.StringLabelFilter:
		return translateLabelsMatcher(t.Matcher)
	case *lokilog.LineFilterLabelFilter:
		return translateLabelsMatcher(t.Matcher)
	default:
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported LogQL label filter %T", f),
		}
	}
}

func translateLabelsMatcher(m *labels.Matcher) (string, error) {
	if m == nil {
		return "", nil
	}
	name := quoteFieldNameIfNeeded(m.Name)
	switch m.Type {
	case labels.MatchEqual:
		return name + ":=" + quoteScalarIfNeeded(m.Value), nil
	case labels.MatchNotEqual:
		return "-" + name + ":=" + quoteScalarIfNeeded(m.Value), nil
	case labels.MatchRegexp:
		return name + ":~" + quoteString(m.Value), nil
	case labels.MatchNotRegexp:
		return "-" + name + ":~" + quoteString(m.Value), nil
	default:
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported LogQL matcher type %v", m.Type),
		}
	}
}

func translateScalarFilter(field string, ty lokilog.LabelFilterType, value string) (string, error) {
	name := quoteFieldNameIfNeeded(field)
	switch ty {
	case lokilog.LabelFilterEqual:
		return name + ":=" + quoteScalarIfNeeded(value), nil
	case lokilog.LabelFilterNotEqual:
		return "-" + name + ":=" + quoteScalarIfNeeded(value), nil
	case lokilog.LabelFilterGreaterThan:
		return name + ":>" + quoteScalarIfNeeded(value), nil
	case lokilog.LabelFilterGreaterThanOrEqual:
		return name + ":>=" + quoteScalarIfNeeded(value), nil
	case lokilog.LabelFilterLesserThan:
		return name + ":<" + quoteScalarIfNeeded(value), nil
	case lokilog.LabelFilterLesserThanOrEqual:
		return name + ":<=" + quoteScalarIfNeeded(value), nil
	default:
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported LogQL label comparison %v", ty),
		}
	}
}

func translateIPFilter(field string, ty lokilog.LabelFilterType, pattern string) (string, error) {
	name := quoteFieldNameIfNeeded(field)
	ipFilter := name + ":ipv4_range(" + quoteString(pattern) + ")"
	switch ty {
	case lokilog.LabelFilterEqual:
		return ipFilter, nil
	case lokilog.LabelFilterNotEqual:
		return "-" + ipFilter, nil
	default:
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: "only '=' and '!=' are supported for LogQL ip() label filter",
		}
	}
}

var lokiTemplateVarRe = regexp.MustCompile(`{{\s*\.\s*([a-zA-Z0-9_.:-]+)\s*}}`)

func convertLokiTemplateToLogsQLPattern(s string) string {
	// Best-effort conversion of `{{.label}}` -> `<label>`.
	return lokiTemplateVarRe.ReplaceAllString(s, `<$1>`)
}

func quoteString(s string) string {
	// Use Go string literal rules, which match LogsQL escaping needs well enough.
	return strconv.Quote(s)
}

func quoteScalarIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	if isBareScalar(s) {
		return s
	}
	return quoteString(s)
}

func quoteFieldNameIfNeeded(name string) string {
	if name == "" {
		return `""`
	}
	if isBareFieldName(name) {
		return name
	}
	return quoteString(name)
}

func isBareFieldName(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' {
			continue
		}
		return false
	}
	return true
}

func isSimpleExtractionField(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-' {
			continue
		}
		return false
	}
	return true
}

func isBareScalar(s string) bool {
	// Allow identifiers and numeric literals without quoting.
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-' || c == ':' || c == '/' {
			continue
		}
		return false
	}
	return true
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func translateSampleExpr(expr syntax.SampleExpr) (string, error) {
	switch e := expr.(type) {
	case *syntax.RangeAggregationExpr:
		return translateRangeAggregation(e, nil)
	case *syntax.VectorAggregationExpr:
		return translateVectorAggregation(e)
	default:
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported LogQL metric expression %T", expr),
		}
	}
}

func translateVectorAggregation(e *syntax.VectorAggregationExpr) (string, error) {
	switch e.Operation {
	case syntax.OpTypeSum:
		r, ok := e.Left.(*syntax.RangeAggregationExpr)
		if !ok {
			return "", &TranslationError{
				Code:    http.StatusBadRequest,
				Message: "only sum(<range_aggregation>) is supported for now",
			}
		}
		return translateRangeAggregation(r, e.Grouping)
	case syntax.OpTypeTopK, syntax.OpTypeBottomK:
		inner, err := translateSampleExpr(e.Left)
		if err != nil {
			return "", err
		}
		if e.Grouping != nil && !e.Grouping.Singleton() {
			return "", &TranslationError{
				Code:    http.StatusBadRequest,
				Message: "topk/bottomk with grouping isn't supported yet",
			}
		}
		order := "value desc"
		if e.Operation == syntax.OpTypeBottomK {
			order = "value"
		}
		return inner + fmt.Sprintf(" | first %d (%s)", e.Params, order), nil
	default:
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported LogQL vector aggregation %q", e.Operation),
		}
	}
}

func translateRangeAggregation(e *syntax.RangeAggregationExpr, grouping *syntax.Grouping) (string, error) {
	sel, err := e.Selector()
	if err != nil {
		return "", newBadRequest("invalid LogQL metric expression", err)
	}

	selector := newLogsQLBuilder()
	var selectorFilters []string
	if tf := timeFilterFromLogRange(e.Left); tf != "" {
		selectorFilters = append(selectorFilters, tf)
	}
	if err := selector.addLogSelectorWithFilters(sel, selectorFilters); err != nil {
		return "", err
	}

	for _, pf := range postFiltersFromUnwrap(e.Left.Unwrap) {
		f, err := translateLabelFilterer(pf)
		if err != nil {
			return "", err
		}
		selector.addFilter(f)
	}

	by := []string{"_stream"}
	if grouping != nil {
		if grouping.Without {
			return "", &TranslationError{
				Code:    http.StatusBadRequest,
				Message: "grouping 'without(...)' isn't supported yet",
			}
		}
		switch {
		case grouping.Singleton():
			by = nil
		case grouping.Noop():
			by = []string{"_stream"}
		default:
			by = grouping.Groups
		}
	}

	stats, err := rangeAggregationToStatsPipe(e)
	if err != nil {
		return "", err
	}

	if len(by) > 0 {
		selector.addPipe(fmt.Sprintf("stats by (%s) %s", strings.Join(by, ", "), stats))
	} else {
		selector.addPipe("stats " + stats)
	}
	return selector.String(), nil
}

func postFiltersFromUnwrap(u *syntax.UnwrapExpr) []lokilog.LabelFilterer {
	if u == nil || len(u.PostFilters) == 0 {
		return nil
	}
	return u.PostFilters
}

func timeFilterFromLogRange(r *syntax.LogRangeExpr) string {
	if r == nil {
		return ""
	}
	filter := "_time:" + prommodel.Duration(r.Interval).String()
	if r.Offset != 0 {
		filter += " offset " + prommodel.Duration(r.Offset).String()
	}
	return filter
}

func rangeAggregationToStatsPipe(e *syntax.RangeAggregationExpr) (string, error) {
	switch e.Operation {
	case syntax.OpRangeTypeRate:
		if e.Left.Unwrap != nil {
			return "", &TranslationError{Code: http.StatusBadRequest, Message: "rate(...| unwrap ...) isn't supported yet"}
		}
		return "rate() as value", nil
	case syntax.OpRangeTypeCount:
		if e.Left.Unwrap != nil {
			return "", &TranslationError{Code: http.StatusBadRequest, Message: "count_over_time(...| unwrap ...) isn't supported yet"}
		}
		return "count() as value", nil
	case syntax.OpRangeTypeAvg, syntax.OpRangeTypeSum, syntax.OpRangeTypeMin, syntax.OpRangeTypeMax:
		if e.Left.Unwrap == nil {
			return "", &TranslationError{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("%s without unwrap isn't supported", e.Operation),
			}
		}
		field := quoteFieldNameIfNeeded(e.Left.Unwrap.Identifier)
		var fn string
		switch e.Operation {
		case syntax.OpRangeTypeAvg:
			fn = "avg(" + field + ")"
		case syntax.OpRangeTypeSum:
			fn = "sum(" + field + ")"
		case syntax.OpRangeTypeMin:
			fn = "min(" + field + ")"
		case syntax.OpRangeTypeMax:
			fn = "max(" + field + ")"
		}
		return fn + " as value", nil
	case syntax.OpRangeTypeQuantile:
		if e.Left.Unwrap == nil || e.Params == nil {
			return "", &TranslationError{Code: http.StatusBadRequest, Message: "quantile_over_time requires unwrap and quantile parameter"}
		}
		field := quoteFieldNameIfNeeded(e.Left.Unwrap.Identifier)
		return fmt.Sprintf("quantile(%s, %s) as value", formatFloat(*e.Params), field), nil
	default:
		return "", &TranslationError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("unsupported LogQL range aggregation %q", e.Operation),
		}
	}
}
