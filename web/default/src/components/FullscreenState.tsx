export function FullscreenState({ title }: { title: string }) {
  return (
    <div
      style={{
        minHeight: "100vh",
        display: "grid",
        placeItems: "center",
        background: "#10131a",
        color: "#e1e2eb",
        fontSize: "18px",
        fontWeight: 600,
        fontFamily: "'Inter', system-ui, sans-serif",
      }}
    >
      {title}
    </div>
  );
}
