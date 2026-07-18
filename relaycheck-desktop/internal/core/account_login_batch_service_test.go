package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRetryPasswordLoginDoesNotReturnInternalErrors(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	service := NewAccountLoginBatchService(app)
	service.passwordLogin = func(_ context.Context, _ *accountAuthContext) error {
		return errors.New(`POST https://relay.example/login: token=TOP_SECRET`)
	}
	result := service.RetryPasswordLogin(t.Context(), "account-1", &accountAuthContext{
		AccountID: "account-1",
		LoginName: "user@example.test",
		Password:  "password",
	})

	if result.Message != "密码登录失败，请检查账号凭据或改用网页登录。" {
		t.Fatalf("unexpected password login public message: %q", result.Message)
	}
	if strings.Contains(result.Message, "TOP_SECRET") || strings.Contains(result.Message, "relay.example") {
		t.Fatalf("password login result leaked an internal error: %q", result.Message)
	}
}
