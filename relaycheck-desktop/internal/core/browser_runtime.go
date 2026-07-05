package core

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// estimateCookieExpiry returns an ISO 8601 timestamp approximately 30 days
// from now, representing the estimated cookie expiry for most relay sites.
func estimateCookieExpiry() string {
	return time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
}

type cdpCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type cdpResponse struct {
	ID     int `json:"id"`
	Result struct {
		Cookies []cdpCookie `json:"cookies"`
		Result  struct {
			Value string `json:"value"`
		} `json:"result"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func readChromeSession(port int) ([]cdpCookie, string, error) {
	pageWS, err := findPageWebSocket(port)
	if err != nil {
		return nil, "", err
	}
	conn, _, err := websocket.DefaultDialer.Dial(pageWS, nil)
	if err != nil {
		return nil, "", err
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]interface{}{"id": 1, "method": "Network.getAllCookies"}); err != nil {
		return nil, "", err
	}
	var cookieResp cdpResponse
	if err := conn.ReadJSON(&cookieResp); err != nil {
		return nil, "", err
	}
	if cookieResp.Error != nil {
		return nil, "", errors.New(cookieResp.Error.Message)
	}

	userAgent := ""
	_ = conn.WriteJSON(map[string]interface{}{
		"id":     2,
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{"expression": "navigator.userAgent", "returnByValue": true},
	})
	var uaResp cdpResponse
	if err := conn.ReadJSON(&uaResp); err == nil {
		userAgent = uaResp.Result.Result.Value
	}

	return cookieResp.Result.Cookies, userAgent, nil
}

func findPageWebSocket(port int) (string, error) {
	// Use a bounded-time HTTP client so a stuck Chrome DevTools endpoint
	// cannot hang the entire saveBrowserLoginSession request indefinitely.
	// The default http.DefaultClient has no timeout.
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/json/list")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var pages []struct {
		Type                 string `json:"type"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &pages); err != nil {
		return "", err
	}
	for _, page := range pages {
		if page.Type == "page" && page.WebSocketDebuggerURL != "" {
			return page.WebSocketDebuggerURL, nil
		}
	}
	return "", errors.New("未找到可读取的浏览器页面，请确认登录页仍然打开。")
}

func buildCookieHeader(cookies []cdpCookie) string {
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name != "" {
			parts = append(parts, cookie.Name+"="+cookie.Value)
		}
	}
	return strings.Join(parts, "; ")
}

func freeDebugPort(used map[int]bool) (int, error) {
	for port := 9222; port < 9250; port++ {
		if used[port] {
			continue
		}
		listener, err := netListen("127.0.0.1:" + strconv.Itoa(port))
		if err == nil {
			_ = listener.Close()
			return port, nil
		}
	}
	return 0, errors.New("没有可用的浏览器调试端口。")
}

func findChrome() (string, error) {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, candidate := range candidates {
		if candidate != "" {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return exec.LookPath("chrome")
}
