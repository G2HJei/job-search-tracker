package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"time"
)

// writeFileAtomic writes data to name (relative to root) so that readers see
// either the old or the new file, never a partial one.
func writeFileAtomic(root *os.Root, name string, data []byte) error {
	return writeAtomic(root, name, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// writeAtomic writes a temp file in the target's folder, syncs and closes
// it, then renames it over the target.
func writeAtomic(root *os.Root, name string, write func(io.Writer) error) (err error) {
	tmp := path.Join(path.Dir(name), ".tmp-"+randomHex(6))
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Close()
			root.Remove(tmp)
		}
	}()
	if err = write(f); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return renameRetry(root, tmp, name)
}

// renameRetry renames with a few retries. On Windows, os.Rename replaces the
// target, but antivirus scanners and the search indexer can hold a file open
// for a moment, which makes the rename fail with "access denied".
func renameRetry(root *os.Root, from, to string) error {
	delay := 10 * time.Millisecond
	var err error
	for range 6 {
		if err = root.Rename(from, to); err == nil || errors.Is(err, fs.ErrNotExist) {
			return err
		}
		time.Sleep(delay)
		delay *= 2
	}
	return err
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
