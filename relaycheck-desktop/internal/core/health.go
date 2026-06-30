package core

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
)

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	status := a.healthStatus(r.Context())
	writeJSON(w, http.StatusOK, status)
}

func (a *App) healthStatus(ctx context.Context) HealthStatus {
	checks := []HealthCheck{
		a.healthCheckDB(ctx),
		healthCheckPath("database", "数据库文件", a.databasePath(), false),
		healthCheckPath("data_dir", "数据目录", a.dataDir, true),
		healthCheckPath("keys_dir", "密钥目录", filepath.Join(a.dataDir, "keys"), true),
		a.healthCheckScheduler(),
		a.healthCheckNotificationChannels(),
	}
	overall := "ok"
	for _, check := range checks {
		if check.Status == "error" {
			overall = "down"
			break
		}
		if check.Status == "warning" && overall == "ok" {
			overall = "degraded"
		}
	}
	return HealthStatus{Status: overall, GeneratedAt: now(), Checks: checks}
}

func (a *App) healthCheckDB(ctx context.Context) HealthCheck {
	if err := a.db.PingContext(ctx); err != nil {
		return HealthCheck{ID: "db", Label: "SQLite 连接", Status: "error", Message: err.Error()}
	}
	var one int
	if err := a.db.QueryRowContext(ctx, `SELECT 1`).Scan(&one); err != nil || one != 1 {
		if err != nil {
			return HealthCheck{ID: "db", Label: "SQLite 连接", Status: "error", Message: err.Error()}
		}
		return HealthCheck{ID: "db", Label: "SQLite 连接", Status: "error", Message: "SELECT 1 返回异常。"}
	}
	return HealthCheck{ID: "db", Label: "SQLite 连接", Status: "ok", Message: "数据库可读写连接正常。"}
}

// healthCheckPath is a pure function: does not access *App state.
func healthCheckPath(id string, label string, path string, wantDir bool) HealthCheck {
	info, err := os.Stat(path)
	if err != nil {
		return HealthCheck{ID: id, Label: label, Status: "error", Message: err.Error()}
	}
	if wantDir && !info.IsDir() {
		return HealthCheck{ID: id, Label: label, Status: "error", Message: "路径存在，但不是目录。"}
	}
	if !wantDir && info.IsDir() {
		return HealthCheck{ID: id, Label: label, Status: "error", Message: "路径存在，但不是文件。"}
	}
	return HealthCheck{ID: id, Label: label, Status: "ok", Message: path}
}

func (a *App) healthCheckScheduler() HealthCheck {
	a.mu.RLock()
	started := !a.schedulerStartedAt.IsZero()
	a.mu.RUnlock()
	if !started {
		return HealthCheck{ID: "scheduler", Label: "后台调度器", Status: "warning", Message: "调度器尚未启动；测试实例可能是预期状态。"}
	}
	return HealthCheck{ID: "scheduler", Label: "后台调度器", Status: "ok", Message: "后台任务循环已启动。"}
}
