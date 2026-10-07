// Package store is the only package that touches the data directory. It
// loads everything into memory at startup, serves reads from memory, and
// writes to disk atomically before updating memory.
package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/G2HJei/job-search-tracker/internal/model"
)

var (
	ErrNotFound = errors.New("not found")
	// ErrConflict means the data changed in the app since the form was opened.
	ErrConflict = errors.New("changed elsewhere since you started editing; reload and try again")
	// ErrExternalChange means the file changed on disk since it was loaded.
	ErrExternalChange = errors.New("the file was changed on disk since it was loaded; reload from disk and try again")
)

const (
	configFile = "config.yaml"
	lockFile   = ".lock"
	companyDir = "companies"
	appDir     = "applications"
	cvDir      = "cv"
	trashDir   = ".trash"
	appFile    = "application.yaml"
	filesDir   = "files"
)

// LoadError describes a file that could not be loaded. Such files are never
// overwritten by the app.
type LoadError struct {
	Path string // relative to the data directory, slash-separated
	Err  string
}

type fileSig struct {
	mod  time.Time
	size int64
}

// Options configures a Store.
type Options struct {
	Now    func() time.Time // defaults to time.Now
	Logger *slog.Logger     // defaults to slog.Default()
}

// Store holds all data in memory, guarded by one RWMutex.
type Store struct {
	dir  string
	root *os.Root
	now  func() time.Time
	log  *slog.Logger

	ownLock     bool
	lockWarning string

	mu        sync.RWMutex
	cfg       model.Config
	cfgRaw    []byte
	apps      map[string]*model.Application
	companies map[string]*model.Company
	loadErrs  []LoadError
	sigs      map[string]fileSig // file → size/mtime when last loaded or written
}

// Open opens (creating if needed) the data directory and loads everything.
func Open(dir string, opts Options) (*Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	s := &Store{dir: abs, root: root, now: opts.Now, log: opts.Logger}
	if s.now == nil {
		s.now = time.Now
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	for _, d := range []string{companyDir, appDir, cvDir, trashDir} {
		if err := root.MkdirAll(d, 0o755); err != nil {
			root.Close()
			return nil, err
		}
	}
	if _, err := root.Stat(configFile); errors.Is(err, fs.ErrNotExist) {
		if err := writeFileAtomic(root, configFile, []byte(model.DefaultConfigYAML)); err != nil {
			root.Close()
			return nil, fmt.Errorf("write default config: %w", err)
		}
		s.log.Info("created default config", "path", filepath.Join(abs, configFile))
	}
	s.acquireLock()
	if err := s.Reload(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// acquireLock writes data/.lock with our PID, warning if one already exists.
func (s *Store) acquireLock() {
	if b, err := s.root.ReadFile(lockFile); err == nil {
		pid := strings.TrimSpace(string(b))
		s.lockWarning = fmt.Sprintf("data/.lock exists (PID %s): another instance may be using this data directory, or the last run did not shut down cleanly.", pid)
		s.log.Warn("lock file exists", "pid", pid)
	}
	if err := s.root.WriteFile(lockFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err == nil {
		s.ownLock = true
	}
}

// Close removes the lock file and releases the directory.
func (s *Store) Close() error {
	if s.ownLock {
		s.root.Remove(lockFile)
		s.ownLock = false
	}
	return s.root.Close()
}

// Dir returns the absolute data directory.
func (s *Store) Dir() string { return s.dir }

// Config returns the current configuration.
func (s *Store) Config() model.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// ConfigYAML returns config.yaml as it is on disk.
func (s *Store) ConfigYAML() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return string(s.cfgRaw)
}

// LoadErrors lists files that could not be loaded.
func (s *Store) LoadErrors() []LoadError {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]LoadError(nil), s.loadErrs...)
}

// Warnings lists other problems worth showing, such as a stale lock file.
func (s *Store) Warnings() []string {
	if s.lockWarning == "" {
		return nil
	}
	return []string{s.lockWarning}
}

// Reload re-reads everything from disk.
func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) load() error {
	l := loader{root: s.root, sigs: map[string]fileSig{}}

	cfgRaw, err := s.root.ReadFile(configFile)
	cfg := model.DefaultConfig()
	if err != nil {
		l.fail(configFile, err)
	} else if cfg, err = model.ParseConfig(cfgRaw); err != nil {
		l.fail(configFile, err)
	}

	companies := map[string]*model.Company{}
	entries, err := fs.ReadDir(s.root.FS(), companyDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".yaml") || strings.HasPrefix(name, ".") {
			continue
		}
		var c model.Company
		if l.decode(path.Join(companyDir, name), &c) {
			c.ID = strings.TrimSuffix(name, ".yaml")
			companies[c.ID] = &c
		}
	}

	apps := map[string]*model.Application{}
	entries, err = fs.ReadDir(s.root.FS(), appDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		var a model.Application
		if l.decode(path.Join(appDir, e.Name(), appFile), &a) {
			a.ID = e.Name()
			apps[a.ID] = &a
		}
	}

	s.cfg, s.cfgRaw = cfg, cfgRaw
	s.companies, s.apps = companies, apps
	s.loadErrs, s.sigs = l.errs, l.sigs
	for _, le := range l.errs {
		s.log.Warn("could not load file", "path", le.Path, "err", le.Err)
	}
	s.log.Debug("loaded data", "dir", s.dir, "applications", len(apps), "companies", len(companies))
	return nil
}

type loader struct {
	root *os.Root
	errs []LoadError
	sigs map[string]fileSig
}

func (l *loader) fail(name string, err error) {
	l.errs = append(l.errs, LoadError{Path: name, Err: err.Error()})
}

// decode reads a YAML file strictly: unknown keys are an error, because
// saving the file again would silently drop them.
func (l *loader) decode(name string, v any) bool {
	info, err := l.root.Stat(name)
	if err != nil {
		l.fail(name, err)
		return false
	}
	b, err := l.root.ReadFile(name)
	if err != nil {
		l.fail(name, err)
		return false
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		l.fail(name, err)
		return false
	}
	var version int
	switch x := v.(type) {
	case *model.Application:
		version = x.SchemaVersion
		x.SchemaVersion = model.SchemaVersion
	case *model.Company:
		version = x.SchemaVersion
		x.SchemaVersion = model.SchemaVersion
	}
	if version > model.SchemaVersion {
		l.fail(name, fmt.Errorf("schemaVersion %d was written by a newer version of the app", version))
		return false
	}
	l.sigs[name] = fileSig{info.ModTime(), info.Size()}
	return true
}

// encodeYAML encodes v with 2-space indentation.
func encodeYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// checkUnchanged returns ErrExternalChange if the file on disk is not the one
// we loaded or last wrote. Caller holds s.mu.
func (s *Store) checkUnchanged(name string) error {
	info, err := s.root.Stat(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrExternalChange
		}
		return err
	}
	if sig, ok := s.sigs[name]; !ok || !sig.mod.Equal(info.ModTime()) || sig.size != info.Size() {
		return ErrExternalChange
	}
	return nil
}

// writeYAML encodes v, writes it atomically and records the new signature.
// Caller holds s.mu.
func (s *Store) writeYAML(name string, v any) error {
	b, err := encodeYAML(v)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(s.root, name, b); err != nil {
		return err
	}
	if info, err := s.root.Stat(name); err == nil {
		s.sigs[name] = fileSig{info.ModTime(), info.Size()}
	}
	return nil
}

// trashName returns a unique path in .trash for something being deleted.
func (s *Store) trashName(name string) string {
	base := path.Join(trashDir, s.now().Format("20060102-150405")+"-"+name)
	return uniqueID(base, func(p string) bool {
		_, err := s.root.Stat(p)
		return err == nil
	})
}

// timestamp returns now truncated to seconds, strictly after prev.
func (s *Store) timestamp(prev time.Time) time.Time {
	t := s.now().Truncate(time.Second)
	if !prev.IsZero() && !t.After(prev) {
		t = prev.Add(time.Second)
	}
	return t
}
