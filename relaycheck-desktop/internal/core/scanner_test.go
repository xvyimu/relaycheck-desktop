package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectUpstreamRecognizesNewAPIPanelSignals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`<html><title>New API</title><body>用户登录 令牌 渠道 额度</body></html>`))
		case "/api/user/self", "/api/channel/", "/api/token/", "/api/status":
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
		case "/v1/models":
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-4o-mini"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := &App{client: server.Client(), allowLocalOutbound: true}
	detection := app.detectUpstream(context.Background(), server.URL)

	if detection.Kind != "newapi" {
		t.Fatalf("expected newapi, got %s with signals %v", detection.Kind, detection.MatchedSignals)
	}
	if detection.HealthStatus != "auth_required" {
		t.Fatalf("expected auth_required, got %s", detection.HealthStatus)
	}
	if detection.DetectionConfidence < 0.7 {
		t.Fatalf("expected high confidence, got %.2f", detection.DetectionConfidence)
	}
}

func TestDetectUpstreamRecognizesSub2APISignals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body>Sub2API subscription API Gateway quota dashboard</body></html>`))
		case "/api/v1/status":
			_, _ = w.Write([]byte(`{"data":{"api_key":"sk-test","quota":100,"subscription":"active"}}`))
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-chat"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := &App{client: server.Client(), allowLocalOutbound: true}
	detection := app.detectUpstream(context.Background(), server.URL)

	if detection.Kind != "sub2api" {
		t.Fatalf("expected sub2api, got %s with signals %v", detection.Kind, detection.MatchedSignals)
	}
	if !detection.SupportsModels {
		t.Fatal("expected models support")
	}
}

func TestDetectUpstreamRecognizesModifiedNewAPIByLoginAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/login":
			http.Error(w, `{"success":false,"message":"missing username or password"}`, http.StatusBadRequest)
		case "/api/user/self":
			http.Error(w, `{"success":false,"message":"unauthorized"}`, http.StatusUnauthorized)
		case "/v1/models":
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"claude-opus-4-6"},{"id":"gpt-5.5"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := &App{client: server.Client(), allowLocalOutbound: true}
	detection := app.detectUpstream(context.Background(), server.URL)

	if detection.Kind != "modified_relay" {
		t.Fatalf("expected modified_relay, got %s with signals %v", detection.Kind, detection.MatchedSignals)
	}
	if detection.HealthStatus != "auth_required" {
		t.Fatalf("expected auth_required, got %s", detection.HealthStatus)
	}
	if !containsString(detection.MatchedSignals, "api-user-login") {
		t.Fatalf("expected api-user-login signal, got %v", detection.MatchedSignals)
	}
}

func TestParseModelIDsAndChooseSpeedTestModel(t *testing.T) {
	models := parseModelIDs(`{"object":"list","data":[{"id":"embedding-1"},{"id":"deepseek-chat"},{"id":"gpt-4o-mini"}]}`)
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d: %v", len(models), models)
	}
	if got := chooseModelForSpeedTest(models); got != "gpt-4o-mini" {
		t.Fatalf("expected preferred speed test model, got %s", got)
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
