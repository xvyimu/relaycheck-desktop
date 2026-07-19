/**
 * First-interactive 计时：进程内 UI 里程碑（RUM 计划的 UI waterfall 缺口）。
 * 全部本地：performance.mark + localStorage，不外发。
 */
const MARK_INTERACTIVE = "relaycheck:first-interactive";
const STORAGE_KEY = "relaycheck_first_interactive_ms";

let marked = false;

/** App 首次脱离 initial loading 时调用一次；重复调用忽略。 */
export function markFirstInteractive(): number | null {
  if (marked || typeof performance === "undefined") return null;
  marked = true;
  const ms = Math.round(performance.now());
  try {
    performance.mark(MARK_INTERACTIVE);
  } catch {
    /* mark 失败不影响业务 */
  }
  try {
    if (typeof window !== "undefined" && window.localStorage) {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ ms, at: new Date().toISOString() }));
    }
  } catch {
    /* storage 拒绝写时静默 */
  }
  // 采样脚本/操作者可从 console 或 localStorage 读取。
  console.info(`[perf] first-interactive ${ms}ms (navigationStart 起)`);
  return ms;
}

/** 读取最近一次 first-interactive 记录（采样脚本用）。 */
export function readFirstInteractive(): { ms: number; at: string } | null {
  try {
    if (typeof window === "undefined" || !window.localStorage) return null;
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as { ms?: number; at?: string };
    if (typeof parsed.ms !== "number") return null;
    return { ms: parsed.ms, at: parsed.at || "" };
  } catch {
    return null;
  }
}

/** 测试注入用：重置单次标记状态。 */
export function resetFirstInteractiveForTest() {
  marked = false;
}
