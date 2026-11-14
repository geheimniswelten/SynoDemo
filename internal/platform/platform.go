// Package platform connects the package service, the DSM CGI and DSM sessions.
package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	packageRoot    = "/var/packages/h5uSynoDemo"
	webPath        = "/webman/3rdparty/h5uSynoDemo/api.cgi"
	localhost      = "http://synodemo"
	metadataHeader = "X-SynoDemo-CGI-Context"
	controlHeader  = "X-SynoDemo-Service-Key"
	maxBody        = 34 << 20
)

var ErrStopped = errors.New("SynoDemo service is stopped")

// DataDir is private to the package account and survives package upgrades.
func DataDir() string {
	if dir := os.Getenv("SYNODEMO_VAR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("SYNOPKG_PKGVAR"); dir != "" {
		return filepath.Join(dir, "private")
	}
	return packageRoot + "/var/private"
}

func socketPath() string {
	if value := os.Getenv("SYNODEMO_SOCKET"); value != "" {
		return value
	}
	return packageRoot + "/target/run/backend.sock"
}

type cgiContext struct {
	ClientIP   string `json:"clientIP"`
	ServerIP   string `json:"serverIP"`
	ServerPort string `json:"serverPort"`
	Scheme     string `json:"scheme"`
}

type contextKey struct{}

func (c cgiContext) valid() bool {
	port, err := strconv.Atoi(c.ServerPort)
	return net.ParseIP(c.ClientIP) != nil &&
		(c.ServerIP == "" || net.ParseIP(c.ServerIP) != nil) &&
		err == nil && port > 0 && port <= 65535 &&
		(c.Scheme == "http" || c.Scheme == "https")
}

func sendError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func unixClient(timeout time.Duration) (*http.Client, func()) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", socketPath())
		},
	}
	client := &http.Client{Transport: transport, Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return client, transport.CloseIdleConnections
}

// CGI relays only the package API. It runs in the web server's CGI identity
// and never reads the persistent package data directory.
func CGI() error {
	return cgi.Serve(http.HandlerFunc(func(w http.ResponseWriter, incoming *http.Request) {
		if incoming.Method != http.MethodGet && incoming.Method != http.MethodPost {
			sendError(w, 405, "method_not_allowed")
			return
		}
		if incoming.ContentLength > maxBody {
			sendError(w, 413, "request_too_large")
			return
		}
		port := os.Getenv("SERVER_PORT")
		scheme := "http"
		if incoming.TLS != nil || incoming.URL.Scheme == "https" || port == "5001" {
			scheme = "https"
		}
		if port == "" {
			_, port, _ = net.SplitHostPort(incoming.Host)
			if port == "" {
				if scheme == "https" {
					port = "5001"
				} else {
					port = "5000"
				}
			}
		}
		metadata := cgiContext{
			ClientIP: os.Getenv("REMOTE_ADDR"), ServerIP: os.Getenv("SERVER_ADDR"),
			ServerPort: port, Scheme: scheme,
		}
		if !metadata.valid() {
			sendError(w, 502, "invalid_dsm_context")
			return
		}
		pathInfo := os.Getenv("PATH_INFO")
		if pathInfo != "" && pathInfo != "/" {
			sendError(w, 404, "not_found")
			return
		}
		url := localhost + "/api.cgi"
		if incoming.URL.RawQuery != "" {
			url += "?" + incoming.URL.RawQuery
		}
		req, err := http.NewRequestWithContext(incoming.Context(), incoming.Method, url, incoming.Body)
		if err != nil {
			sendError(w, 400, "invalid_request")
			return
		}
		req.ContentLength = incoming.ContentLength
		req.Host = incoming.Host
		for _, key := range []string{"Cookie", "Accept-Language", "Content-Type", "Origin", "Sec-Fetch-Site", "X-SYNO-TOKEN", "X-SynoDemo-CSRF"} {
			req.Header[key] = incoming.Header.Values(key)
		}
		data, _ := json.Marshal(metadata)
		req.Header.Set(metadataHeader, string(data))
		client, cleanup := unixClient(60 * time.Second)
		defer cleanup()
		res, err := client.Do(req)
		if err != nil {
			sendError(w, 503, "package_unavailable")
			return
		}
		defer res.Body.Close()
		for _, key := range []string{"Content-Type", "Content-Language", "Content-Disposition", "Cache-Control", "X-Content-Type-Options"} {
			if value := res.Header.Get(key); value != "" {
				w.Header().Set(key, value)
			}
		}
		w.WriteHeader(res.StatusCode)
		_, _ = io.Copy(w, res.Body)
	}))
}

// Identity separates a stable DSM user ID from the current session binding.
type Identity struct {
	User    string
	Session string
}

// Authorize checks a DSM session and restricts the package to local DSM
// administrators. The application receives a digest of the cookie and token.
func Authorize(r *http.Request) (Identity, bool) {
	meta, ok := r.Context().Value(contextKey{}).(cgiContext)
	cookie := r.Header.Get("Cookie")
	token := r.Header.Get("X-SYNO-TOKEN")
	if !ok || !meta.valid() || cookie == "" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return Identity{}, false
	}
	env := []string{
		"PATH=/usr/syno/bin:/usr/syno/sbin:/usr/bin:/usr/sbin:/bin:/sbin",
		"GATEWAY_INTERFACE=CGI/1.1",
		"HTTP_COOKIE=" + cookie,
		"HTTP_X_SYNO_TOKEN=" + token,
		"REMOTE_ADDR=" + meta.ClientIP,
		"SERVER_ADDR=" + meta.ServerIP,
		"SERVER_PORT=" + meta.ServerPort,
		"HTTP_HOST=" + r.Host,
		"REQUEST_METHOD=" + r.Method,
		"QUERY_STRING=" + r.URL.RawQuery,
		"SCRIPT_NAME=" + webPath,
		"REQUEST_URI=" + webPath + "?" + r.URL.RawQuery,
		"SERVER_PROTOCOL=" + r.Proto,
		"REQUEST_SCHEME=" + meta.Scheme,
	}
	if meta.Scheme == "https" {
		env = append(env, "HTTPS=on")
	} else {
		env = append(env, "HTTPS=off")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	for _, executable := range []string{"/usr/syno/synoman/webman/authenticate.cgi", "/usr/syno/synoman/webman/modules/authenticate.cgi"} {
		cmd := exec.CommandContext(ctx, executable)
		cmd.Env = env
		output, err := cmd.Output()
		if err != nil {
			continue
		}
		user := strings.TrimSpace(string(output))
		if user == "" || len(user) > 256 || strings.ContainsAny(user, "\r\n\x00") {
			continue
		}
		groups, err := exec.CommandContext(ctx, "id", "-Gn", "--", user).Output()
		if err != nil {
			return Identity{}, false
		}
		for _, group := range strings.Fields(string(groups)) {
			if group == "administrators" {
				session := sha256.Sum256([]byte(user + "\x00" + cookie + "\x00" + token))
				return Identity{User: user, Session: hex.EncodeToString(session[:])}, true
			}
		}
		return Identity{}, false
	}
	return Identity{}, false
}

// Control supports the package lifecycle entry points. The stop command uses a
// private credential even though DSM's CGI can connect to the same local socket.
func Control(action string) error {
	switch action {
	case "init":
		return ensureDataDir()
	case "status":
		client, cleanup := unixClient(2 * time.Second)
		defer cleanup()
		res, err := client.Get(localhost + "/internal/health")
		if err != nil {
			return ErrStopped
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			return ErrStopped
		}
		return nil
	case "stop":
		return stopService()
	default:
		return fmt.Errorf("unknown package command %q", action)
	}
}

func ensureDataDir() error {
	dir := DataDir()
	if !filepath.IsAbs(dir) {
		return errors.New("package data directory must be absolute")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.Chmod(dir, 0700)
}

func keyFile() string {
	return filepath.Join(DataDir(), "service-key")
}

func stopService() error {
	if err := Control("status"); errors.Is(err, ErrStopped) {
		if _, statErr := os.Stat(DataDir()); os.IsNotExist(statErr) {
			return nil
		}
		return waitForLock()
	}
	token, err := os.ReadFile(keyFile())
	if os.IsNotExist(err) {
		return waitForLock()
	}
	if err != nil {
		return err
	}
	client, cleanup := unixClient(3 * time.Second)
	defer cleanup()
	req, err := http.NewRequest(http.MethodPost, localhost+"/internal/stop", nil)
	if err != nil {
		return err
	}
	req.Header.Set(controlHeader, string(token))
	res, err := client.Do(req)
	if err != nil {
		if errors.Is(Control("status"), ErrStopped) {
			return waitForLock()
		}
		return err
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("package service rejected stop: HTTP %d", res.StatusCode)
	}
	return waitForLock()
}

func waitForLock() error {
	deadline := time.Now().Add(8 * time.Second)
	for {
		unlock, err := lockService()
		if err == nil {
			unlock()
			return nil
		}
		if !errors.Is(err, errAlreadyRunning) || time.Now().After(deadline) {
			return fmt.Errorf("package service did not stop: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func listen() (net.Listener, error) {
	socket := socketPath()
	if !filepath.IsAbs(socket) {
		return nil, errors.New("package socket path must be absolute")
	}
	parent := filepath.Dir(socket)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return nil, err
	}
	if err := os.Chmod(parent, 0755); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(socket); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, errors.New("package socket path is occupied")
		}
		if err := os.Remove(socket); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socket, 0666); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

// Serve starts the unprivileged backend on the package Unix socket.
func Serve(createHandler func(context.Context) (http.Handler, func(), error)) error {
	if err := ensureDataDir(); err != nil {
		return err
	}
	unlock, err := lockService()
	if err != nil {
		return err
	}
	defer unlock()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	token := hex.EncodeToString(key)
	if err := os.WriteFile(keyFile(), []byte(token), 0600); err != nil {
		return err
	}
	defer os.Remove(keyFile())

	listener, err := listen()
	if err != nil {
		return err
	}
	defer listener.Close()
	ctx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()
	ctx, stop := context.WithCancel(ctx)
	handler, cleanup, err := createHandler(ctx)
	defer func() {
		stop()
		if cleanup != nil {
			cleanup()
		}
	}()
	if err != nil {
		return err
	}
	router := http.NewServeMux()
	router.HandleFunc("GET /internal/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.HandleFunc("POST /internal/stop", func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(controlHeader)), []byte(token)) != 1 {
			sendError(w, 403, "invalid_control_key")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		stop()
	})
	router.HandleFunc("/api.cgi", func(w http.ResponseWriter, r *http.Request) {
		var metadata cgiContext
		raw := r.Header.Get(metadataHeader)
		if len(raw) > 1024 || json.Unmarshal([]byte(raw), &metadata) != nil || !metadata.valid() {
			sendError(w, 403, "invalid_dsm_context")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			sendError(w, 405, "method_not_allowed")
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), contextKey{}, metadata))
		r.RemoteAddr = metadata.ClientIP
		r.URL.Scheme = metadata.Scheme
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		handler.ServeHTTP(w, r)
	})
	server := &http.Server{
		Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second,
		WriteTimeout: 60 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 32 << 10,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
		}
	}()
	err = server.Serve(listener)
	stop()
	<-done
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
