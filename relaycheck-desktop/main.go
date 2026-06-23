package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"relaycheck-desktop/internal/core"
	"relaycheck-desktop/internal/lock"
)

//go:embed frontend/dist
var staticFiles embed.FS

func main() {
	app, err := core.NewApp(".")
	if err != nil {
		log.Fatal(err)
	}
	// ── Duplicate-instance guard ──────────────────────────────────
	lockFile := filepath.Join(app.DataDir(), ".lock")
	lk, err := lock.Acquire(lockFile)
	if err != nil {
		log.Fatalf("WARN: %v (lockfile: %s)", err, lockFile)
	}
	defer lk.Close()
	defer app.Close()
	// ──────────────────────────────────────────────────────────────

	mux := http.NewServeMux()
	app.RegisterRoutes(mux)
	registerStatic(mux)

	bind := "127.0.0.1"
	preferredPort := envInt("RELAYCHECK_PORT", 3001)
	listener, actualPort, err := listenWithFallback(bind, preferredPort)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	app.SetRuntimeAddress(bind, actualPort)
	app.StartSchedulers(context.Background())

	addr := bind + ":" + strconv.Itoa(actualPort)
	server := &http.Server{
		Addr:              addr,
		Handler:           app.SecureLocalHandler(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	url := "http://" + addr
	_ = openURL(url)
	log.Printf("RelayCheck Desktop running at %s", url)

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func registerStatic(mux *http.ServeMux) {
	dist, err := fs.Sub(staticFiles, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(dist))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/" {
			if _, err := fs.Stat(dist, r.URL.Path[1:]); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		index, err := staticFiles.ReadFile("frontend/dist/index.html")
		if err != nil {
			http.Error(w, "frontend is not built", http.StatusInternalServerError)
			return
		}
		w.Header().Set("content-type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
}

func openURL(url string) error {
	if os.Getenv("RELAYCHECK_NO_OPEN") == "1" {
		return nil
	}

	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func listenWithFallback(bind string, preferredPort int) (net.Listener, int, error) {
	candidates := []int{preferredPort, 3001, 3010, 3000, 3002, 3003, 8080, 9999, 7897}
	seen := map[int]bool{}

	for _, port := range candidates {
		if port <= 0 || seen[port] {
			continue
		}
		seen[port] = true
		listener, err := net.Listen("tcp", bind+":"+strconv.Itoa(port))
		if err == nil {
			if port != preferredPort {
				log.Printf("port %d is busy, using %d", preferredPort, port)
			}
			return listener, port, nil
		}
	}

	for port := 3011; port < 3030; port++ {
		if seen[port] {
			continue
		}
		listener, err := net.Listen("tcp", bind+":"+strconv.Itoa(port))
		if err == nil {
			log.Printf("port %d is busy, using %d", preferredPort, port)
			return listener, port, nil
		}
	}

	return nil, 0, os.ErrPermission
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
