import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table.tsx";
import { useMemo } from "react";

export interface QueryResultsTableProps {
  readonly data?: unknown;
}

export function QueryResultsTable({ data }: QueryResultsTableProps) {
  const rows = useMemo(() => extractRows(data), [data]);

  if (!rows || rows.length === 0) {
    return (
      <div className="table-wrapper">
        <div className="text-sm text-slate-400">
          No tabular rows to display.
        </div>
      </div>
    );
  }

  const columns = Array.from(
    rows.reduce((set, row) => {
      Object.keys(row || {}).forEach((key) => set.add(key));
      return set;
    }, new Set<string>()),
  );

  return (
    <Table>
      <TableHeader>
        <TableRow>
          {columns.map((col) => (
            <TableHead key={col}>{col}</TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {(!rows || rows.length === 0) && (
          <div className="text-sm text-slate-400">
            No tabular rows to display.
          </div>
        )}
        {rows.map((row, idx) => (
          <TableRow key={idx}>
            {columns.map((col) => (
              <TableCell key={col}>{renderCellValue(row[col])}</TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

function extractRows(value: unknown): Array<Record<string, unknown>> {
  if (typeof value === "string") {
    const parsed = parseDataString(value);
    if (parsed !== undefined) {
      value = parsed;
    }
  }

  if (!value) return [];

  if (isPrometheusResponse(value)) {
    return prometheusResponseToRows(value);
  }

  if (Array.isArray(value)) return value as Array<Record<string, unknown>>;

  if (typeof value === "object") {
    const asRecord = value as Record<string, unknown>;
    if (Array.isArray(asRecord.data))
      return asRecord.data as Array<Record<string, unknown>>;
    if (Array.isArray(asRecord.result))
      return asRecord.result as Array<Record<string, unknown>>;
  }

  return [];
}

function renderCellValue(v: unknown): any {
  if (v === null || v === undefined) return "";
  if (typeof v === "string" || typeof v === "number" || typeof v === "boolean")
    return String(v);
  return (
    <pre className="whitespace-pre-wrap break-words text-xs">
      {JSON.stringify(v, null, 2)}
    </pre>
  );
}

function parseDataString(s: string): unknown | undefined {
  const trimmed = s.trim();
  if (!trimmed) return undefined;

  // Try plain JSON first (VictoriaLogs stats endpoints return JSON objects).
  try {
    return JSON.parse(trimmed);
  } catch {
    // fallthrough
  }

  // Then try NDJSON (VictoriaLogs query endpoint returns one JSON object per line).
  const lines = trimmed.split("\n").map((line) => line.trim());
  const objs: unknown[] = [];
  for (const line of lines) {
    if (!line) continue;
    try {
      objs.push(JSON.parse(line));
    } catch {
      return undefined;
    }
  }
  return objs;
}

type PrometheusResponse = {
  status?: string;
  data?: {
    resultType?: string;
    result?: unknown[];
  };
};

function isPrometheusResponse(v: unknown): v is PrometheusResponse {
  if (!v || typeof v !== "object") return false;
  const r = v as Record<string, unknown>;
  if (!("data" in r)) return false;
  const data = r.data as any;
  return (
    !!data &&
    typeof data === "object" &&
    typeof data.resultType === "string" &&
    Array.isArray(data.result)
  );
}

function prometheusResponseToRows(resp: PrometheusResponse): Array<Record<string, unknown>> {
  const resultType = resp.data?.resultType;
  const result = resp.data?.result ?? [];

  if (resultType === "matrix") {
    return result.flatMap((series: any) => {
      const metric: Record<string, string> = series?.metric ?? {};
      const values: Array<[number, string]> = series?.values ?? [];
      const last = values.length > 0 ? values[values.length - 1] : undefined;
      const ts = last ? last[0] : undefined;
      const val = last ? last[1] : undefined;
      return [
        {
          ...metric,
          timestamp: ts,
          value: val,
          samples: values.length,
        },
      ];
    });
  }

  if (resultType === "vector") {
    return result.flatMap((series: any) => {
      const metric: Record<string, string> = series?.metric ?? {};
      const value: [number, string] | undefined = series?.value;
      return [
        {
          ...metric,
          timestamp: value?.[0],
          value: value?.[1],
        },
      ];
    });
  }

  // Fallback: show raw items.
  if (Array.isArray(result)) {
    return result as Array<Record<string, unknown>>;
  }
  return [];
}
