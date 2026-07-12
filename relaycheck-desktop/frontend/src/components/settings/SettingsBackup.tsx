import type { SystemBackup } from "@/types";
import { formatBytes, formatTime } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";

export type SettingsBackupProps = {
  backups: SystemBackup[];
  busy: boolean;
  multiSelect: boolean;
  selected: string[];
  onRefresh: () => void;
  onToggleMulti: () => void;
  onToggleSelect: (fileName: string) => void;
  onDeleteSelected: () => void;
  onRestore: (backup: SystemBackup) => void;
};

export function SettingsBackup({
  backups,
  busy,
  multiSelect,
  selected,
  onRefresh,
  onToggleMulti,
  onToggleSelect,
  onDeleteSelected,
  onRestore,
}: SettingsBackupProps) {
  return (
    <article className="card">
      <div className="section-heading">
        <div>
          <strong>备份快照</strong>
          <span>{multiSelect ? "已选择 " + selected.length + " 个备份" : "默认突出最新一个；可打开多选清理旧快照。"}</span>
        </div>
        <div className="toolbar compact-toolbar">
          <Button variant="ghost" disabled={busy} onClick={onRefresh}>
            刷新
          </Button>
          <Button variant={multiSelect ? "default" : "ghost"} disabled={busy} onClick={onToggleMulti}>
            {multiSelect ? "退出多选" : "多选管理"}
          </Button>
          {multiSelect ? (
            <button className="danger" disabled={busy || !selected.length} onClick={() => void onDeleteSelected()}>
              {busy ? "删除中…" : "删除选中"}
            </button>
          ) : null}
        </div>
      </div>
      <div className="list compact">
        {(multiSelect ? backups.slice(0, 24) : backups.slice(0, 1)).map((backup, index) => (
          <article
            className={
              "detail-row backup-row " +
              (index === 0 ? "is-latest" : "") +
              " " +
              (selected.includes(backup.fileName) ? "is-selected" : "")
            }
            key={backup.fileName}
          >
            {multiSelect ? (
              <label className="backup-check">
                <input type="checkbox" checked={selected.includes(backup.fileName)} onChange={() => onToggleSelect(backup.fileName)} />
              </label>
            ) : null}
            <div>
              <strong>
                {backup.fileName}
                {index === 0 ? " · 最新" : ""}
              </strong>
              <span>
                {formatBytes(backup.sizeBytes)} {"·"} {formatTime(backup.createdAt)}
              </span>
            </div>
            <button className="danger" disabled={busy} onClick={() => void onRestore(backup)}>
              {busy ? "恢复中…" : "恢复"}
            </button>
          </article>
        ))}
        {!backups.length ? <EmptyState title="暂无备份" description='点击"立即备份数据库"后，这里会出现可恢复的本地快照。' /> : null}
      </div>
    </article>
  );
}