export function EmptyState({ title, description }: { title: string; description: string }) {
  return (
    <div className="empty-state">
      <div className="empty-mark">RC</div>
      <strong>{title}</strong>
      <span>{description}</span>
    </div>
  );
}