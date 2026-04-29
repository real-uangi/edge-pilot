import { ResponsiveLine } from "@nivo/line";
import styles from "./UnifiedLineChart.module.css";

export type ChartPoint = { x: string | number | Date; y: number | null };

export type ChartSeries = {
  id: string;
  color?: string;
  data: ChartPoint[];
};

type YFormatter = (value: number) => string;

export interface UnifiedLineChartProps {
  title?: string;
  legend?: boolean;
  series?: ChartSeries[];
  data?: ChartPoint[];
  id?: string;
  color?: string;
  xAxisMode?: "time" | "index";
  yUnit?: string;
  yValueFormatter?: YFormatter;
  tooltipValueFormatter?: YFormatter;
  height?: number;
  showGridX?: boolean;
  emptyMessage?: string;
}

const chartTheme = {
  axis: {
    ticks: {
      line: { stroke: "rgba(60,73,78,0.3)" },
      text: { fill: "#bbc9cf", fontSize: 11 },
    },
    legend: {
      text: { fill: "#859399", fontSize: 11 },
    },
  },
  grid: {
    line: { stroke: "rgba(60,73,78,0.2)" },
  },
  crosshair: {
    line: {
      stroke: "#a4e6ff",
      strokeWidth: 1.5,
      strokeOpacity: 0.95,
      strokeDasharray: "4 3",
    },
  },
  tooltip: {
    container: {
      background: "transparent",
      border: "0",
      borderRadius: "0",
      boxShadow: "none",
      padding: "0",
    },
  },
} as const;

const defaultPalette = ["#00d1ff", "#4edea3", "#ffb778", "#ff8e89", "#b5a4ff"];

export function UnifiedLineChart({
  title,
  legend,
  series,
  data,
  id = "series",
  color,
  xAxisMode = "time",
  yUnit,
  yValueFormatter,
  tooltipValueFormatter,
  height = 260,
  showGridX = false,
  emptyMessage = "暂无趋势数据",
}: UnifiedLineChartProps) {
  const normalizedSeries = normalizeSeries({ series, data, id, color, xAxisMode });
  const seriesMeta = normalizedSeries.map((item, index) => ({
    id: String(item.id),
    color: item.color ?? defaultPalette[index % defaultPalette.length],
  }));
  const hasData = normalizedSeries.some((item) => item.data.length > 0);
  const showLegend = legend ?? normalizedSeries.length > 1;

  if (!hasData) {
    return (
      <div className={styles.card}>
        {title ? <div className={styles.title}>{title}</div> : null}
        <div className={styles.empty}>{emptyMessage}</div>
      </div>
    );
  }

  const formatY = (value: number) => {
    if (yValueFormatter) {
      return yValueFormatter(value);
    }
    if (yUnit) {
      return `${value}${yUnit}`;
    }
    return String(value);
  };

  const formatTooltipY = (value: number) => {
    if (tooltipValueFormatter) {
      return tooltipValueFormatter(value);
    }
    return formatY(value);
  };

  return (
    <div className={styles.card}>
      {title || showLegend ? (
        <div className={styles.header}>
          {title ? <div className={styles.title}>{title}</div> : <span />}
          {showLegend ? (
            <div className={styles.legend}>
              {normalizedSeries.map((item, index) => (
                <span className={styles.legendItem} key={String(item.id)}>
                  <span
                    className={styles.legendSwatch}
                    style={{ backgroundColor: seriesMeta[index].color }}
                  />
                  {String(item.id)}
                </span>
              ))}
            </div>
          ) : null}
        </div>
      ) : null}

      <div className={styles.wrap} style={{ height }}>
        <ResponsiveLine
          data={normalizedSeries}
          margin={{ top: 12, right: 16, bottom: 28, left: 44 }}
          curve="monotoneX"
          xScale={
            xAxisMode === "time"
              ? { type: "time", format: "native", precision: "second", useUTC: false }
              : { type: "point" }
          }
          xFormat={xAxisMode === "time" ? "time:%Y-%m-%d %H:%M:%S" : undefined}
          yScale={{ type: "linear", min: 0, max: "auto", stacked: false, reverse: false }}
          axisTop={null}
          axisRight={null}
          axisBottom={{
            tickSize: 0,
            tickPadding: 8,
            tickRotation: 0,
            tickValues: xAxisMode === "time" ? 5 : [],
            format: (value) => formatXAxisLabel(value, xAxisMode),
          }}
          axisLeft={{
            tickSize: 0,
            tickPadding: 8,
            tickRotation: 0,
            format: (value) => formatY(Number(value)),
          }}
          enablePoints={false}
          lineWidth={2}
          colors={seriesMeta.map((item) => item.color)}
          enableGridX={showGridX}
          enableSlices="x"
          useMesh
          theme={chartTheme}
          sliceTooltip={(slice) => renderSliceTooltip(slice, xAxisMode, formatTooltipY, seriesMeta)}
        />
      </div>
    </div>
  );
}

function renderSliceTooltip(
  slice: unknown,
  xAxisMode: "time" | "index",
  valueFormatter: YFormatter,
  seriesMeta: Array<{ id: string; color: string }>,
) {
  const payload = slice as {
    points?: Array<{
      data?: { x?: unknown; xFormatted?: unknown; y?: unknown; yFormatted?: unknown };
      serieId?: string | number;
      serie?: { id?: string | number };
      color?: string;
    }>;
    slice?: {
      points?: Array<{
        data?: { x?: unknown; xFormatted?: unknown; y?: unknown; yFormatted?: unknown };
        serieId?: string | number;
        serie?: { id?: string | number };
        color?: string;
      }>;
    };
  };
  const points = payload.slice?.points ?? payload.points ?? [];
  const xValue = points[0]?.data?.x ?? points[0]?.data?.xFormatted;
  const xLabel = formatXAxisLabel(xValue, xAxisMode);
  const rows = points
    .map((point, index) => {
      const rawValue = point.data?.y ?? point.data?.yFormatted;
      const yValue = typeof rawValue === "number" ? rawValue : Number(rawValue);
      const yLabel = Number.isFinite(yValue) ? valueFormatter(yValue) : "—";
      const seriesLabel = resolveSeriesLabel(point, index, seriesMeta);
      const color = resolveSeriesColor(point, index, seriesMeta);
      return { seriesLabel, color, yLabel };
    })
    .sort((a, b) => compareAscii(a.seriesLabel, b.seriesLabel));

  return (
    <div className={styles.tooltipCard}>
      <div className={styles.tooltipMeta}>{xLabel}</div>
      <div className={styles.tooltipRows}>
        {rows.map((row, index) => (
          <div className={styles.tooltipRow} key={`${row.seriesLabel}-${index}`}>
            <div className={styles.tooltipHead}>
              <span className={styles.tooltipDot} style={{ backgroundColor: row.color }} />
              <span>{row.seriesLabel}</span>
            </div>
            <div className={styles.tooltipValue}>{row.yLabel}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

function resolveSeriesLabel(
  point: { serieId?: string | number; serie?: { id?: string | number }; color?: string },
  index: number,
  seriesMeta: Array<{ id: string; color: string }>,
): string {
  const raw = String(point.serieId ?? point.serie?.id ?? "").trim();
  if (raw && raw !== "series") {
    return raw;
  }
  if (point.color) {
    const byColor = seriesMeta.find((item) => item.color.toLowerCase() === point.color!.toLowerCase());
    if (byColor) {
      return byColor.id;
    }
  }
  return seriesMeta[index]?.id ?? "series";
}

function resolveSeriesColor(
  point: { color?: string },
  index: number,
  seriesMeta: Array<{ id: string; color: string }>,
): string {
  if (point.color) {
    return point.color;
  }
  return seriesMeta[index]?.color ?? defaultPalette[index % defaultPalette.length];
}

function compareAscii(a: string, b: string): number {
  if (a === b) {
    return 0;
  }
  return a < b ? -1 : 1;
}

function formatXAxisLabel(value: unknown, xAxisMode: "time" | "index"): string {
  if (xAxisMode === "index") {
    return String(value ?? "");
  }
  const date = parseAsDate(value);
  if (!date) {
    return "—";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(date);
}

function parseAsDate(value: unknown): Date | null {
  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? null : value;
  }
  if (typeof value === "string" || typeof value === "number") {
    const next = new Date(value);
    return Number.isNaN(next.getTime()) ? null : next;
  }
  return null;
}

function normalizeSeries({
  series,
  data,
  id,
  color,
  xAxisMode,
}: {
  series?: ChartSeries[];
  data?: ChartPoint[];
  id: string;
  color?: string;
  xAxisMode: "time" | "index";
}) {
  const rawSeries = series?.length ? series : [{ id, color, data: data ?? [] }];
  return rawSeries.map((item) => ({
    id: item.id,
    color: item.color,
    data: item.data
      .map((point, index) => ({
        x: normalizeX(point.x, index, xAxisMode),
        y: point.y,
      }))
      .filter((point) => point.x !== null),
  }));
}

function normalizeX(value: ChartPoint["x"], index: number, xAxisMode: "time" | "index") {
  if (xAxisMode === "index") {
    if (value == null) {
      return String(index + 1);
    }
    return String(value);
  }
  const parsed = parseAsDate(value);
  return parsed;
}
