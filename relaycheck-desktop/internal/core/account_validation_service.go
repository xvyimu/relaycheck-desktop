package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type AccountValidationService struct {
	db          *sql.DB
	accountAuth *AccountAuthRepository
	accountAPI  *AccountAPIClient
}

func NewAccountValidationService(app *App) *AccountValidationService {
	return &AccountValidationService{
		db:          app.db,
		accountAuth: app.accountAuth,
		accountAPI:  app.accountAPI,
	}
}

type accountLoginTestResult struct {
	Status     string `json:"status"`
	HTTPStatus int    `json:"httpStatus"`
}

type accountValidationHTTPError struct {
	status  int
	message string
}

func (e accountValidationHTTPError) Error() string {
	return e.message
}

func accountValidationHTTPErrorStatus(err error) (int, string) {
	if typed, ok := err.(accountValidationHTTPError); ok {
		return typed.status, typed.message
	}
	return http.StatusInternalServerError, err.Error()
}

func (s *AccountValidationService) TestLogin(ctx context.Context, id string) (accountLoginTestResult, error) {
	auth, err := s.accountAuth.Load(ctx, id)
	if err != nil {
		if err.Error() == accountAuthNotFoundMessage {
			return accountLoginTestResult{}, accountValidationHTTPError{status: http.StatusNotFound, message: err.Error()}
		}
		return accountLoginTestResult{}, err
	}
	if _, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizeBaseURL(auth.BaseURL)+"/api/user/self", nil); err != nil {
		return accountLoginTestResult{}, accountValidationHTTPError{status: http.StatusBadRequest, message: "账号 Base URL 无效，无法测试登录态。"}
	}

	result := accountLoginTestResult{Status: "unknown"}
	status, _, err := s.accountAPI.Do(ctx, *auth, http.MethodGet, "/api/user/self", nil)
	if err == nil {
		result.HTTPStatus = status
		if status == http.StatusOK {
			result.Status = "valid"
		} else if status == http.StatusUnauthorized || status == http.StatusForbidden {
			result.Status = "expired"
		}
	}
	if _, execErr := s.db.ExecContext(ctx, `UPDATE channel_accounts SET login_status=?, last_validated_at=?, updated_at=? WHERE id=?`, result.Status, now(), now(), id); execErr != nil {
		log.Printf("[accounts] test login status update failed for account %s: %v", id, execErr)
	}
	return result, nil
}

func (s *AccountValidationService) TestAPIKey(ctx context.Context, id string, auth *accountAuthContext) apiKeyTestResult {
	if auth == nil {
		loaded, err := s.accountAuth.Load(ctx, id)
		if err != nil {
			return apiKeyTestResult{AccountID: id, Status: "failed", Message: err.Error()}
		}
		auth = loaded
	}
	result := apiKeyTestResult{
		AccountID:   auth.AccountID,
		AccountName: auth.AccountName,
		SiteName:    auth.UpstreamSite,
		Fingerprint: secretFingerprint(auth.APIKey),
	}
	if strings.TrimSpace(auth.APIKey) == "" {
		result.Status = "missing"
		result.Message = "该账号没有保存 API Key。"
		return result
	}
	auth.Cookie = ""
	auth.AccessToken = ""
	auth.AuthUserID = ""

	modelsStatus, modelsBody, modelsErr := s.accountAPI.Do(ctx, *auth, http.MethodGet, "/v1/models", nil)
	result.HTTPStatus = modelsStatus
	result.Path = "/v1/models"
	if modelsErr != nil {
		result.Status = "unknown"
		result.Message = modelsErr.Error()
	} else if modelsStatus == http.StatusUnauthorized || modelsStatus == http.StatusForbidden {
		result.Status = "expired"
		result.Message = firstNonEmpty(extractMessage(modelsBody), "API Key 无权访问 /v1/models。")
	} else if modelsStatus >= 200 && modelsStatus < 300 {
		models := parseModelIDs(modelsBody)
		result.Status = "valid"
		result.ModelCount = len(models)
		result.SampleModels = limitStrings(models, 8)
		result.Message = fmt.Sprintf("/v1/models 返回 HTTP %d，识别到 %d 个模型。", modelsStatus, len(models))
		if len(models) > 0 {
			result.TestedModel = chooseModelForSpeedTest(models)
			s.SpeedTestAPIKeyModel(ctx, auth, &result)
		} else {
			result.ModelTestMessage = "模型列表为空，未执行可用性测速。"
		}
	} else if modelsStatus == http.StatusNotFound || modelsStatus == http.StatusMethodNotAllowed {
		result.Status = "unknown"
		result.Message = firstNonEmpty(extractMessage(modelsBody), "/v1/models 不可用，继续用面板接口判断 Key。")
	} else {
		result.Status = "unknown"
		result.Message = firstNonEmpty(extractMessage(modelsBody), fmt.Sprintf("/v1/models 返回 HTTP %d。", modelsStatus))
	}

	if result.Status == "unknown" {
		probes := []string{"/api/user/self", "/api/token/"}
		for _, path := range probes {
			status, body, err := s.accountAPI.Do(ctx, *auth, http.MethodGet, path, nil)
			if err != nil {
				result.Path = path
				result.Message = err.Error()
				continue
			}
			result.HTTPStatus = status
			result.Path = path
			result.Message = firstNonEmpty(extractMessage(body), fmt.Sprintf("%s 返回 HTTP %d", path, status))
			if status == http.StatusOK {
				result.Status = "valid"
				break
			}
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				result.Status = "expired"
				break
			}
			if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
				continue
			}
			if status >= 200 && status < 300 {
				result.Status = "valid"
				break
			}
		}
	}
	if result.Status == "" {
		result.Status = "unknown"
		result.Message = "没有找到可判断 API Key 的接口。"
	}
	if result.Status == "valid" && result.ModelCount > 0 && result.TestedModel != "" {
		if result.ModelUsable {
			result.Message = fmt.Sprintf("密钥有效，模型 %s 可用，测速 %dms。", result.TestedModel, result.ModelTestLatencyMs)
		} else if result.ModelTestMessage != "" {
			result.Message = "密钥可读取模型，但模型调用未通过：" + result.ModelTestMessage
		}
	}
	result.Message = sanitizeAPIKeyDiagnostic(result.Message, auth.APIKey)
	result.ModelTestMessage = sanitizeAPIKeyDiagnostic(result.ModelTestMessage, auth.APIKey)
	sampleModelsJSON := marshalStringSlice(limitStrings(result.SampleModels, 8))
	if _, execErr := s.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET api_key_fingerprint=?, api_key_status=?, api_key_last_checked_at=?,
		    api_key_model_count=?, api_key_sample_models_json=?, api_key_test_model=?,
		    api_key_model_usable=?, api_key_latency_ms=?, api_key_test_http_status=?,
		    api_key_test_message=?, api_key_test_path=?,
		    login_status=CASE WHEN ?='valid' THEN 'valid' WHEN ?='expired' THEN 'expired' ELSE login_status END,
		    last_validated_at=?, updated_at=?
		WHERE id=?
	`, result.Fingerprint, result.Status, now(), result.ModelCount, sampleModelsJSON, result.TestedModel, boolInt(result.ModelUsable), result.ModelTestLatencyMs, result.ModelTestHTTPStatus, result.ModelTestMessage, result.ModelTestPath, result.Status, result.Status, now(), now(), id); execErr != nil {
		log.Printf("[accounts] api key test result update failed for account %s: %v", id, execErr)
	}
	return result
}

func (s *AccountValidationService) SpeedTestAPIKeyModel(ctx context.Context, auth *accountAuthContext, result *apiKeyTestResult) {
	if strings.TrimSpace(result.TestedModel) == "" {
		return
	}
	payload := map[string]interface{}{
		"model":       result.TestedModel,
		"messages":    []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens":  1,
		"temperature": 0,
		"stream":      false,
	}
	body, _ := json.Marshal(payload)
	started := time.Now()
	status, responseBody, err := s.accountAPI.DoWithTimeout(ctx, *auth, http.MethodPost, "/v1/chat/completions", body, 12*time.Second)
	result.ModelTestLatencyMs = time.Since(started).Milliseconds()
	result.ModelTestHTTPStatus = status
	result.ModelTestPath = "/v1/chat/completions"
	if err != nil {
		result.ModelTestMessage = err.Error()
		return
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		result.Status = "expired"
		result.ModelTestMessage = firstNonEmpty(extractMessage(responseBody), "模型调用未授权。")
		return
	}
	if status < 200 || status >= 300 {
		result.ModelTestMessage = firstNonEmpty(extractMessage(responseBody), fmt.Sprintf("模型调用返回 HTTP %d。", status))
		return
	}
	if responseExplicitlyFailed(responseBody) {
		result.ModelTestMessage = firstNonEmpty(extractMessage(responseBody), "模型调用返回失败。")
		return
	}
	result.ModelUsable = true
	result.ModelTestMessage = firstNonEmpty(extractMessage(responseBody), "模型调用成功。")
}
