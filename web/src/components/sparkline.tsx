/** A tiny inline chart of daily visitors. Pure SVG, rendered on the server. */
export function Sparkline({ values, label }: { values: number[]; label: string }) {
  const w = 120;
  const h = 32;
  const max = Math.max(1, ...values);
  const step = values.length > 1 ? w / (values.length - 1) : w;
  const points = values.map(
    (v, i) => `${(i * step).toFixed(1)},${(h - 2 - (v / max) * (h - 4)).toFixed(1)}`,
  );
  const line = points.join(" ");
  const area = `0,${h} ${line} ${w},${h}`;
  return (
    <svg
      viewBox={`0 0 ${w} ${h}`}
      className="h-8 w-28"
      role="img"
      aria-label={label}
      preserveAspectRatio="none"
    >
      <polygon points={area} style={{ fill: "var(--chart-fill)" }} />
      <polyline
        points={line}
        fill="none"
        strokeWidth="1.75"
        strokeLinejoin="round"
        strokeLinecap="round"
        style={{ stroke: "var(--chart-line)" }}
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
