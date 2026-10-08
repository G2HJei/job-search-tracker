// Package web serves the UI: routes, handlers, form binding, htmx-aware
// rendering and security middleware.
package web

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/store"
)

// maxUpload caps attachment and CV uploads.
const maxUpload = 25 << 20

// Options configures the server.
type Options struct {
	Store   *store.Store
	Static  fs.FS  // contents of web/static
	Dev     bool   // no caching of static files
	Version string // shown in settings
	Addr    string // listen address; its host is added to the Host allowlist
	Logger  *slog.Logger
	Now     func() time.Time
}

// Server holds the handlers' dependencies.
type Server struct {
	store   *store.Store
	static  fs.FS
	dev     bool
	version string
	log     *slog.Logger
	now     func() time.Time

	hosts    map[string]bool
	anyIP    bool // listening on all interfaces: accept IP-literal Host headers
	assetVer string
}

// New creates a server.
func New(o Options) *Server {
	s := &Server{
		store:   o.Store,
		static:  o.Static,
		dev:     o.Dev,
		version: o.Version,
		log:     o.Logger,
		now:     o.Now,
		hosts:   map[string]bool{"127.0.0.1": true, "localhost": true, "::1": true},
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.now == nil {
		s.now = time.Now
	}
	if host, _, err := net.SplitHostPort(o.Addr); err == nil {
		if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
			s.anyIP = true
		} else {
			s.hosts[strings.ToLower(host)] = true
		}
	}
	s.assetVer = contentVersion(s.static)
	return s
}

// contentVersion hashes every static file, so asset URLs change whenever an
// asset does, even between builds sharing a version string (go run is "dev").
func contentVersion(fsys fs.FS) string {
	if fsys == nil {
		return fmt.Sprint(time.Now().Unix())
	}
	h := sha256.New()
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, path)
		fmt.Fprintf(h, "%s\x00%d\x00", path, len(b))
		h.Write(b)
		return err
	})
	if err != nil {
		return fmt.Sprint(time.Now().Unix())
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// Handler returns the full handler chain.
func (s *Server) Handler() http.Handler {
	var h http.Handler = s.routes()
	h = securityHeaders(h)
	h = http.NewCrossOriginProtection().Handler(h)
	h = s.checkHost(h)
	h = s.logRequests(h)
	return s.recoverPanics(h)
}

func (s *Server) today() model.Date { return model.Today(s.now) }

// checkHost blocks DNS rebinding: a page on evil.example that resolves to
// 127.0.0.1 sends "Host: evil.example", which is rejected.
func (s *Server) checkHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.ToLower(strings.Trim(host, "[]"))
		if !s.hosts[host] && !(s.anyIP && net.ParseIP(host) != nil) {
			http.Error(w, "Host not allowed", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// csp allows only the app's own scripts and styles. Pico's icons are data:
// SVGs, hence img-src data:.
const csp = "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; " +
	"object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		level := slog.LevelDebug
		if sw.status >= 500 {
			level = slog.LevelError
		} else if sw.status >= 400 {
			level = slog.LevelWarn
		}
		s.log.Log(r.Context(), level, "request", "method", r.Method, "path", r.URL.Path,
			"status", sw.status, "duration", time.Since(start).Round(time.Microsecond))
	})
}

func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				s.log.Error("panic", "path", r.URL.Path, "err", v)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// staticHandler serves embedded assets (or files from disk with --dev).
func (s *Server) staticHandler() http.Handler {
	files := http.StripPrefix("/static/", http.FileServerFS(s.static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.dev {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		files.ServeHTTP(w, r)
	})
}
