export function Empty({ message = "暂无数据" }: { message?: string }) {
  return <div className="empty">{message}</div>;
}
