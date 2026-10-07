// Package demo embeds a fake data directory used by --demo and by tests.
package demo

import (
	"bytes"
	"embed"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/G2HJei/job-search-tracker/internal/model"
)

//go:embed sample
var files embed.FS

// Today is the date the sample data was written for. Tests that use FS
// directly should use it as their fixed clock.
var Today = model.NewDate(2026, 10, 7)

// FS returns the sample data directory.
func FS() fs.FS {
	sub, err := fs.Sub(files, "sample")
	if err != nil {
		panic(err)
	}
	return sub
}

// CopyTo copies the sample data into dir, which must be empty or missing,
// and moves every date by the number of days between Today and today, so
// the dashboard looks the same whenever the demo is run.
func CopyTo(dir string, today model.Date) error {
	if err := os.CopyFS(dir, FS()); err != nil {
		return err
	}
	shift := Today.DaysUntil(today)
	if shift == 0 {
		return nil
	}
	paths, err := filepath.Glob(filepath.Join(dir, "applications", "*", "application.yaml"))
	if err != nil {
		return err
	}
	for _, p := range paths {
		if err := shiftFile(p, shift); err != nil {
			return err
		}
	}
	return nil
}

func shiftFile(p string, days int) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	var a model.Application
	if err := yaml.Unmarshal(b, &a); err != nil {
		return err
	}
	a.ShiftDates(days)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(a); err != nil {
		return err
	}
	enc.Close()
	return os.WriteFile(p, buf.Bytes(), 0o644)
}
