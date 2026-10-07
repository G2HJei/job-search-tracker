// Command jedediah serves the Jedediah web UI on 127.0.0.1 and
// stores data as YAML files in a local folder.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/G2HJei/job-search-tracker/internal/demo"
	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/store"
	"github.com/G2HJei/job-search-tracker/internal/web"
	webassets "github.com/G2HJei/job-search-tracker/web"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "jedediah:", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", "127.0.0.1:8765", "listen address")
	dataFlag := flag.String("data", "", "data directory (default: $JEDEDIAH_DATA_DIR, else data/ next to the executable)")
	openFlag := flag.Bool("open", true, "open the default browser once the server is ready (--open=false to skip)")
	dev := flag.Bool("dev", false, "serve static files from disk without caching, and log debug messages")
	demoFlag := flag.Bool("demo", false, "run on a temporary copy of the built-in sample data")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return nil
	}

	level := slog.LevelInfo
	if *dev {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	// Bind first, so a second instance fails before touching the data directory.
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w\nIs Jedediah already running? Open http://%s", *addr, err, *addr)
	}
	defer ln.Close()

	dir, cleanup, err := dataDir(*dataFlag, *demoFlag, log)
	if err != nil {
		return err
	}
	defer cleanup()

	st, err := store.Open(dir, store.Options{Logger: log})
	if err != nil {
		return fmt.Errorf("open data directory %s: %w", dir, err)
	}
	defer st.Close()

	static := webassets.Static()
	if *dev {
		if _, err := os.Stat(filepath.Join("web", "static")); err == nil {
			static = os.DirFS(filepath.Join("web", "static"))
		} else {
			log.Warn("--dev: web/static not found in the working directory; using embedded assets")
		}
	}

	srv := web.New(web.Options{Store: st, Static: static, Dev: *dev, Version: version, Addr: *addr, Logger: log})
	httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}

	url := "http://" + browserAddr(ln.Addr().(*net.TCPAddr))
	fmt.Printf("\n  Jedediah %s is running at %s\n  Data: %s\n  Press Ctrl+C to stop.\n\n", version, url, st.Dir())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- httpSrv.Serve(ln) }()
	if *openFlag {
		go openWhenReady(ctx, url, log)
	}

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}

// dataDir resolves the data directory: --demo → temp copy of the sample
// data; else --data; else $JEDEDIAH_DATA_DIR; else data/ next to the executable.
func dataDir(flagDir string, useDemo bool, log *slog.Logger) (string, func(), error) {
	noop := func() {}
	if useDemo {
		tmp, err := os.MkdirTemp("", "jedediah-demo-")
		if err != nil {
			return "", noop, err
		}
		dir := filepath.Join(tmp, "data")
		if err := demo.CopyTo(dir, model.Today(time.Now)); err != nil {
			os.RemoveAll(tmp)
			return "", noop, fmt.Errorf("prepare demo data: %w", err)
		}
		log.Info("demo mode: using a temporary copy of the sample data; changes are discarded on exit", "dir", dir)
		return dir, func() { os.RemoveAll(tmp) }, nil
	}
	if flagDir != "" {
		return flagDir, noop, nil
	}
	if env := os.Getenv("JEDEDIAH_DATA_DIR"); env != "" {
		return env, noop, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", noop, err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if strings.Contains(exe, "go-build") {
		log.Warn("running from a temporary build folder (go run?); pass --data ./data to keep your data")
	}
	return filepath.Join(filepath.Dir(exe), "data"), noop, nil
}

// browserAddr turns the listen address into one a browser can open.
func browserAddr(a *net.TCPAddr) string {
	if a.IP == nil || a.IP.IsUnspecified() {
		return fmt.Sprintf("127.0.0.1:%d", a.Port)
	}
	return a.String()
}

// openWhenReady waits for /healthz, then opens the default browser.
func openWhenReady(ctx context.Context, url string, log *slog.Logger) {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return
		}
		if resp, err := client.Get(url + "/healthz"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if err := openBrowser(url); err != nil {
					log.Warn("could not open the browser", "err", err)
				}
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Warn("server did not become ready; open the URL yourself", "url", url)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
