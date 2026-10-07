package store

import (
	"archive/zip"
	"cmp"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"
)

// FileInfo describes an attachment or CV file.
type FileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
}

var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// SanitizeFilename reduces name to its base name made of [A-Za-z0-9._-].
// Spaces become hyphens; leading dots and hyphens are removed so the result
// can't be a hidden or temp file.
func SanitizeFilename(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		default:
			b.WriteString(translit[unicode.ToLower(r)])
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.TrimRight(out, ".")
	ext := path.Ext(out)
	if len(ext) > 10 {
		ext = ""
	}
	stem := strings.TrimLeft(strings.TrimSuffix(out, ext), ".-")
	if len(stem) > 90 {
		stem = stem[:90]
	}
	if stem == "" || reservedNames[strings.ToLower(stem)] {
		stem = "file-" + stem
	}
	return strings.TrimSuffix(stem, "-") + ext
}

// validName reports whether name is already a sanitized file name, so it
// can be used to look up a file without any path tricks.
func validName(name string) bool {
	return name != "" && SanitizeFilename(name) == name
}

func filesPath(appID string) string { return path.Join(appDir, appID, filesDir) }

func (s *Store) requireApp(appID string) error {
	if _, ok := s.apps[appID]; !ok {
		return ErrNotFound
	}
	return nil
}

func (s *Store) listDir(dir string) ([]FileInfo, error) {
	entries, err := fs.ReadDir(s.root.FS(), dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []FileInfo
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, FileInfo{Name: e.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	slices.SortFunc(out, func(a, b FileInfo) int { return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out, nil
}

// saveInto writes r to dir under a sanitized, unused version of name and
// returns the final name.
func (s *Store) saveInto(dir, name string, r io.Reader) (string, error) {
	name = SanitizeFilename(name)
	if err := s.root.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ext := path.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	final := uniqueID(stem, func(n string) bool {
		_, err := s.root.Stat(path.Join(dir, n+ext))
		return err == nil
	}) + ext
	err := writeAtomic(s.root, path.Join(dir, final), func(w io.Writer) error {
		_, err := io.Copy(w, r)
		return err
	})
	return final, err
}

// ListFiles lists an application's attachments.
func (s *Store) ListFiles(appID string) ([]FileInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.requireApp(appID); err != nil {
		return nil, err
	}
	return s.listDir(filesPath(appID))
}

// SaveFile stores an uploaded attachment. If the name is taken, -2, -3, …
// is added before the extension. It returns the final file name.
func (s *Store) SaveFile(appID, name string, r io.Reader) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireApp(appID); err != nil {
		return "", err
	}
	return s.saveInto(filesPath(appID), name, r)
}

// ReplaceFile writes an attachment with exactly this name, moving any
// existing file with the name to .trash first.
func (s *Store) ReplaceFile(appID, name string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireApp(appID); err != nil {
		return err
	}
	if !validName(name) {
		return ErrNotFound
	}
	dir := filesPath(appID)
	if err := s.root.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	p := path.Join(dir, name)
	if _, err := s.root.Stat(p); err == nil {
		if err := renameRetry(s.root, p, s.trashName(appID+"-"+name)); err != nil {
			return err
		}
	}
	return writeFileAtomic(s.root, p, data)
}

// OpenFile opens an attachment for reading.
func (s *Store) OpenFile(appID, name string) (*os.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.requireApp(appID); err != nil {
		return nil, err
	}
	return s.openIn(filesPath(appID), name)
}

func (s *Store) openIn(dir, name string) (*os.File, error) {
	if !validName(name) {
		return nil, ErrNotFound
	}
	f, err := s.root.Open(path.Join(dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if info, err := f.Stat(); err != nil || info.IsDir() {
		f.Close()
		return nil, ErrNotFound
	}
	return f, nil
}

// DeleteFile moves an attachment to .trash.
func (s *Store) DeleteFile(appID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireApp(appID); err != nil {
		return err
	}
	if !validName(name) {
		return ErrNotFound
	}
	err := renameRetry(s.root, path.Join(filesPath(appID), name), s.trashName(appID+"-"+name))
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	}
	return err
}

// ListCVs lists the files in the CV library (data/cv).
func (s *Store) ListCVs() ([]FileInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listDir(cvDir)
}

// SaveCV adds a file to the CV library and returns its final name.
func (s *Store) SaveCV(name string, r io.Reader) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveInto(cvDir, name, r)
}

// OpenCV opens a CV file for reading.
func (s *Store) OpenCV(name string) (*os.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.openIn(cvDir, name)
}

// WriteBackup writes a zip of the whole data directory to w.
func (s *Store) WriteBackup(w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	zw := zip.NewWriter(w)
	fsys := s.root.FS()
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." || d.IsDir() {
			return nil
		}
		if p == lockFile || strings.HasPrefix(d.Name(), ".tmp-") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = p
		hdr.Method = zip.Deflate
		dst, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := fsys.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(dst, f)
		return err
	})
	if err != nil {
		return err
	}
	return zw.Close()
}
