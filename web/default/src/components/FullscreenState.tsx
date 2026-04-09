export function FullscreenState({ title }: { title: string }) {
  return (
    <div
      style={{
        minHeight: "100vh",
        display: "grid",
        placeItems: "center",
        background: "#fff",
        color: "#000",
        fontSize: "18px",
        fontWeight: 600,
      }}
    >
      {title}
    </div>
  );
}
