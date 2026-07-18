package core

import (
	"errors"
	"net/http"
	"strings"

	"relaycheck-desktop/internal/accounts"
)

type importFailureMessages struct {
	PathRejected        string
	InvalidFormat       string
	UpstreamAuth        string
	UpstreamUnavailable string
}

func writeImportFailure(w http.ResponseWriter, err error, messages importFailureMessages) {
	status := http.StatusBadRequest
	message := "导入失败，请检查输入后重试。"
	switch {
	case errors.Is(err, accounts.ErrSQLitePathRejected):
		message = firstImportMessage(messages.PathRejected, "导入来源路径不被允许。")
	case errors.Is(err, accounts.ErrImportInvalidFormat):
		message = firstImportMessage(messages.InvalidFormat, "导入文件格式或结构无效。")
	case errors.Is(err, accounts.ErrImportUpstreamAuth):
		message = firstImportMessage(messages.UpstreamAuth, "上游认证失败，请检查访问令牌和权限。")
	case errors.Is(err, accounts.ErrImportUpstreamUnavailable):
		message = firstImportMessage(messages.UpstreamUnavailable, "暂时无法读取上游数据，请稍后重试。")
	case errors.Is(err, accounts.ErrImportStorage):
		status = http.StatusInternalServerError
		message = "服务暂时不可用，请稍后重试。"
	default:
		status = http.StatusInternalServerError
		message = "服务暂时不可用，请稍后重试。"
	}
	writePublicError(w, status, message, err)
}

func firstImportMessage(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
