package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"synodemo/internal/platform"
	"time"
)

type settings struct {
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"`
}
type app struct {
	mu       sync.Mutex
	key      [32]byte
	dir      string
	settings settings
}

func (a *app) log(s string) {
	f, e := os.OpenFile(filepath.Join(a.dir, "events.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e == nil {
		defer f.Close()
		fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), s)
	}
}
func (a *app) csrf(s string) string {
	h := hmac.New(sha256.New, a.key[:])
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
func (a *app) handle(w http.ResponseWriter, r *http.Request) {
	id, ok := platform.Authorize(r)
	if !ok {
		http.Error(w, "unauthorized", 403)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == "GET" {
		log, _ := os.ReadFile(filepath.Join(a.dir, "events.log"))
		if len(log) > 65536 {
			log = log[len(log)-65536:]
		}
		json.NewEncoder(w).Encode(map[string]any{"settings": a.settings, "log": string(log), "csrf": a.csrf(id.Session)})
		return
	}
	if r.Method != "POST" || !hmac.Equal([]byte(a.csrf(id.Session)), []byte(r.Header.Get("X-SynoDemo-CSRF"))) {
		http.Error(w, "forbidden", 403)
		return
	}
	var s settings
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	dec.DisallowUnknownFields()
	if dec.Decode(&s) != nil || (s.Interval != "minute" && s.Interval != "five" && s.Interval != "hour" && s.Interval != "day") {
		http.Error(w, "invalid settings", 400)
		return
	}
	data, _ := json.Marshal(s)
	tmp := filepath.Join(a.dir, "settings.tmp")
	if os.WriteFile(tmp, data, 0600) != nil || os.Rename(tmp, filepath.Join(a.dir, "settings.json")) != nil {
		http.Error(w, "storage error", 500)
		return
	}
	a.settings = s
	a.log("Settings changed by " + id.User)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
func main() {
	if filepath.Base(os.Args[0]) == "api.cgi" || os.Getenv("GATEWAY_INTERFACE") != "" {
		if e := platform.CGI(); e != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "init" || os.Args[1] == "status" || os.Args[1] == "stop") {
		e := platform.Control(os.Args[1])
		if errors.Is(e, platform.ErrStopped) {
			os.Exit(3)
		}
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) != 2 || os.Args[1] != "--serve" {
		fmt.Fprintln(os.Stderr, "usage: synodemo --serve|init|status|stop")
		os.Exit(1)
	}
	e := platform.Serve(func(ctx context.Context) (http.Handler, func(), error) {
		a := &app{dir: platform.DataDir(), settings: settings{Interval: "five"}}
		if e := os.MkdirAll(a.dir, 0700); e != nil {
			return nil, nil, e
		}
		if _, e := rand.Read(a.key[:]); e != nil {
			return nil, nil, e
		}
		if b, e := os.ReadFile(filepath.Join(a.dir, "settings.json")); e == nil {
			_ = json.Unmarshal(b, &a.settings)
		}
		a.log("Service started")
		done := make(chan struct{})
		go func() {
			defer close(done)
			ticker := time.NewTicker(time.Second * 10)
			defer ticker.Stop()
			last := ""
			for {
				select {
				case <-ctx.Done():
					return
				case t := <-ticker.C:
					a.mu.Lock()
					key := t.Format("2006-01-02 15:04")
					due := a.settings.Interval == "minute" || a.settings.Interval == "five" && t.Minute()%5 == 0 || a.settings.Interval == "hour" && t.Minute() == 0 || a.settings.Interval == "day" && t.Hour() == 0 && t.Minute() == 0
					if a.settings.Enabled && due && key != last {
						a.log("Scheduled demo event triggered")
						last = key
					}
					a.mu.Unlock()
				}
			}
		}()
		return http.HandlerFunc(a.handle), func() { <-done; a.mu.Lock(); a.log("Service stopped"); a.mu.Unlock() }, nil
	})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
