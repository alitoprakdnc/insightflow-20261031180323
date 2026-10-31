import React, { useEffect, useState, useCallback, useRef } from "react";
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  Legend,
  ReferenceDot,
  CartesianGrid,
} from "recharts";

// Types
interface KpiMetric {
  id: string;
  name: string;
}

interface KpiRecord {
  timestamp: string; // ISO 8601
  value: number;
  anomaly?: boolean;
}

// Helper to format timestamps for the X‑axis
const formatTimestamp = (iso: string) => {
  const d = new Date(iso);
  return d.toLocaleString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    month: "short",
    day: "numeric",
  });
};

// Custom dot to highlight anomalies
const AnomalyDot: React.FC<{
  cx?: number;
  cy?: number;
  payload?: KpiRecord;
}> = ({ cx, cy, payload }) => {
  if (!payload?.anomaly) return null;
  return (
    <ReferenceDot
      x={cx}
      y={cy}
      r={6}
      fill="#ff4d4f"
      stroke="#fff"
      strokeWidth={2}
    />
  );
};

const Dashboard: React.FC = () => {
  const apiBase = process.env.REACT_APP_API_URL ?? "";
  const [metrics, setMetrics] = useState<KpiMetric[]>([]);
  const [selectedMetric, setSelectedMetric] = useState<string>("");
  const [data, setData] = useState<KpiRecord[]>([]);
  const [loadingMetrics, setLoadingMetrics] = useState<boolean>(true);
  const [loadingData, setLoadingData] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const refreshInterval = useRef<NodeJS.Timeout | null>(null);

  // Abort controllers for fetch cancellation
  const metricsAbortCtrl = useRef<AbortController | null>(null);
  const dataAbortCtrl = useRef<AbortController | null>(null);

  // Fetch list of available KPI metrics
  const fetchMetrics = useCallback(async () => {
    setLoadingMetrics(true);
    setError(null);
    metricsAbortCtrl.current?.abort();
    const controller = new AbortController();
    metricsAbortCtrl.current = controller;

    try {
      const resp = await fetch(`${apiBase}/api/kpis/metrics`, {
        signal: controller.signal,
        credentials: "include",
      });
      if (!resp.ok) {
        throw new Error(`Failed to load metrics: ${resp.status}`);
      }
      const json: KpiMetric[] = await resp.json();
      setMetrics(json);
      if (json.length > 0 && !selectedMetric) {
        setSelectedMetric(json[0].id);
      }
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoadingMetrics(false);
    }
  }, [apiBase, selectedMetric]);

  // Fetch KPI data for the selected metric
  const fetchKpiData = useCallback(async () => {
    if (!selectedMetric) return;
    setLoadingData(true);
    setError(null);
    dataAbortCtrl.current?.abort();
    const controller = new AbortController();
    dataAbortCtrl.current = controller;

    try {
      const resp = await fetch(
        `${apiBase}/api/kpis?metric=${encodeURIComponent(selectedMetric)}`,
        {
          signal: controller.signal,
          credentials: "include",
        }
      );
      if (!resp.ok) {
        throw new Error(`Failed to load KPI data: ${resp.status}`);
      }
      const json: KpiRecord[] = await resp.json();
      // Ensure timestamps are sorted
      const sorted = [...json].sort(
        (a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
      );
      setData(sorted);
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoadingData(false);
    }
  }, [apiBase, selectedMetric]);

  // Initial load of metrics
  useEffect(() => {
    fetchMetrics();
    // Cleanup on unmount
    return () => {
      metricsAbortCtrl.current?.abort();
    };
  }, [fetchMetrics]);

  // Load KPI data whenever the selected metric changes
  useEffect(() => {
    fetchKpiData();
    // Cleanup previous request
    return () => {
      dataAbortCtrl.current?.abort();
    };
  }, [fetchKpiData]);

  // Auto‑refresh KPI data every 30 seconds
  useEffect(() => {
    if (!selectedMetric) return;
    refreshInterval.current = setInterval(fetchKpiData, 30_000);
    return () => {
      if (refreshInterval.current) clearInterval(refreshInterval.current);
    };
  }, [selectedMetric, fetchKpiData]);

  // CSV export helper
  const exportCsv = () => {
    if (data.length === 0) return;
    const header = "timestamp,value,anomaly\n";
    const rows = data
      .map(
        (r) =>
          `${r.timestamp},${r.value},${r.anomaly ? "true" : "false"}`
      )
      .join("\n");
    const blob = new Blob([header + rows], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${selectedMetric}_kpi.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div style={{ padding: "1rem" }}>
      <h2>KPI Dashboard</h2>

      {/* Metric selector */}
      <div style={{ marginBottom: "1rem" }}>
        <label htmlFor="metric-select">Metric:&nbsp;</label>
        {loadingMetrics ? (
          <span>Loading metrics…</span>
        ) : (
          <select
            id="metric-select"
            value={selectedMetric}
            onChange={(e) => setSelectedMetric(e.target.value)}
            disabled={loadingMetrics || metrics.length === 0}
          >
            {metrics.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
        )}
        <button
          onClick={exportCsv}
          disabled={data.length === 0}
          style={{ marginLeft: "1rem" }}
        >
          Export CSV
        </button>
      </div>

      {/* Error handling */}
      {error && (
        <div style={{ color: "red", marginBottom: "1rem" }}>
          Error: {error}
        </div>
      )}

      {/* Loading indicator */}
      {loadingData ? (
        <p>Loading KPI data…</p>
      ) : data.length === 0 ? (
        <p>No data available for the selected metric.</p>
      ) : (
        <ResponsiveContainer width="100%" height={400}>
          <LineChart data={data}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis
              dataKey="timestamp"
              tickFormatter={formatTimestamp}
              minTickGap={20}
            />
            <YAxis domain={["auto", "auto"]} />
            <Tooltip
              labelFormatter={(value) => `Time: ${formatTimestamp(value)}`}
              formatter={(value, name) => [
                typeof value === "number" ? value.toFixed(2) : value,
                name,
              ]}
            />
            <Legend />
            <Line
              type="monotone"
              dataKey="value"
              name="Value"
              stroke="#1890ff"
              dot={false}
              isAnimationActive={false}
            />
            {/* Render anomaly dots */}
            {data.map((d, i) =>
              d.anomaly ? (
                <AnomalyDot
                  key={`anomaly-${i}`}
                  cx={undefined}
                  cy={undefined}
                  payload={d}
                />
              ) : null
            )}
          </LineChart>
        </ResponsiveContainer>
      )}
    </div>
  );
};

export default Dashboard;