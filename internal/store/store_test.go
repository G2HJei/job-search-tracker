package store

import (
	"archive/zip"
	"bytes"
	"errors"
	"flag"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/G2HJei/job-search-tracker/internal/model"
)

var update = flag.Bool("update", false, "rewrite golden files")

var bst = time.FixedZone("BST", 3600)

// clock returns a fake now() that advances by step on every call.
func clock(start time.Time, step time.Duration) func() time.Time {
	t := start
	return func() time.Time {
		now := t
		t = t.Add(step)
		return now
	}
}

func openTest(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir, Options{Now: clock(time.Date(2026, 10, 7, 15, 20, 0, 0, bst), time.Second), Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// exampleApplication mirrors the example in Plan.MD §4.
func exampleApplication() model.Application {
	return model.Application{
		Priority:  "high",
		Status:    "interviewing",
		CompanyID: "acme",
		Job: model.Job{
			Title:            "Senior Backend Engineer",
			PostingURL:       "https://acme.example/careers/123",
			PostingCopy:      "posting.pdf",
			JobID:            "R-12345",
			Department:       "Payments Platform",
			Location:         "London (hybrid, 2 days)",
			KeyRequirements:  "- 5+ years Go or Java\n- Kubernetes, Postgres\n",
			Fit:              4,
			Interest:         5,
			Channel:          "Referral",
			CVVersion:        "cv-backend-v3.pdf",
			ExtraMaterials:   []string{"cover-letter.pdf", "https://github.com/me/portfolio"},
			AdvertisedSalary: model.SalaryRange{Min: 80000, Max: 95000, Currency: "GBP", Period: "year"},
		},
		Contacts: model.Contacts{
			Referrer: &model.Contact{Name: "Jane Smith", Role: "Staff Engineer", Email: "jane@example.com"},
			Recruiters: []model.Contact{{Name: "Tom Lee", Role: "Talent Partner", Email: "tom@acme.example",
				LinkedIn: "https://linkedin.com/in/tomlee"}},
			HiringManager: &model.Contact{Name: "Priya Patel", Role: "Engineering Manager"},
			Interviewers:  []model.Contact{{Name: "Alex Kim", Role: "Senior Engineer"}},
		},
		Process: model.Process{
			NextStep:        "Technical interview (live coding)",
			NextStepDate:    model.NewDate(2026, 10, 14),
			LastContactDate: model.NewDate(2026, 10, 9),
			DateApplied:     model.NewDate(2026, 10, 7),
			ScreeningDate:   model.NewDateTime(2026, 10, 9, 10, 0),
			Interviews: []model.Interview{{Date: model.NewDateTime(2026, 10, 14, 14, 0), Type: "Technical",
				Interviewers: []string{"Alex Kim"}}},
			FollowUpDate: model.NewDate(2026, 10, 16),
			QA: []model.QA{{Round: 0, Question: "Why are you leaving your current role?",
				Answer: "Looking for larger-scale distributed systems work…\n"}},
			Decision: "Pending",
		},
		Compensation: model.Compensation{
			Asked:        model.SalaryRange{Min: 92000, Currency: "GBP", Period: "year"},
			ContractType: "Permanent",
			NoticePeriod: "1 month",
		},
		Other: model.Other{Notes: "Strong engineering blog; ask about on-call.\n"},
	}
}

func TestOpenCreatesLayout(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	for _, p := range []string{"config.yaml", "companies", "applications", "cv", ".trash", ".lock"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
	if got := s.ConfigYAML(); got != model.DefaultConfigYAML {
		t.Errorf("config.yaml not the default")
	}
	if s.Config().StaleAfterDays != 14 || len(s.LoadErrors()) != 0 {
		t.Errorf("config=%+v errs=%v", s.Config(), s.LoadErrors())
	}
	s.Close()
	if _, err := os.Stat(filepath.Join(dir, ".lock")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf(".lock not removed on close")
	}
}

func TestLockWarning(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".lock"), []byte("4242"), 0o644)
	s := openTest(t, dir)
	if w := s.Warnings(); len(w) != 1 || !strings.Contains(w[0], "4242") {
		t.Fatalf("warnings = %v", w)
	}
}

func TestApplicationRoundTripGolden(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	if _, err := s.CreateCompany(model.Company{Name: "Acme", Industry: "Fintech", Size: "201-1000",
		HQ: "London", Website: "https://acme.example"}); err != nil {
		t.Fatal(err)
	}
	a, err := s.CreateApplication(exampleApplication())
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "2026-10-07-acme-senior-backend-engineer" {
		t.Fatalf("id = %q", a.ID)
	}
	got, err := os.ReadFile(filepath.Join(dir, "applications", a.ID, "application.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "application.golden.yaml")
	if *update {
		os.WriteFile(golden, got, 0o644)
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))) {
		t.Fatalf("application.yaml differs from golden (run with -update to accept):\n%s", got)
	}

	// A fresh store reads back exactly what was written.
	s.Close()
	s2 := openTest(t, dir)
	back, ok := s2.GetApplication(a.ID)
	if !ok {
		t.Fatalf("not loaded; errors: %v", s2.LoadErrors())
	}
	if model.Rev(back) != model.Rev(a) {
		t.Fatalf("round trip changed data:\n%+v\n%+v", back, a)
	}
	c, ok := s2.GetCompany("acme")
	if !ok || c.Name != "Acme" || c.Size != "201-1000" {
		t.Fatalf("company = %+v", c)
	}
}

func TestSlugCollisions(t *testing.T) {
	s := openTest(t, t.TempDir())
	c1, _ := s.CreateCompany(model.Company{Name: "Acme Ltd."})
	c2, _ := s.CreateCompany(model.Company{Name: "ACME ltd"})
	c3, _ := s.CreateCompany(model.Company{Name: "Телерик & Co"})
	if c1.ID != "acme-ltd" || c2.ID != "acme-ltd-2" || c3.ID != "telerik-and-co" {
		t.Fatalf("company ids: %q %q %q", c1.ID, c2.ID, c3.ID)
	}
	var ids []string
	for range 3 {
		a, err := s.CreateApplication(model.Application{CompanyID: c1.ID, Job: model.Job{Title: "Go Engineer (Remote)"}})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
	}
	want := []string{"2026-10-07-acme-ltd-go-engineer-remote", "2026-10-07-acme-ltd-go-engineer-remote-2", "2026-10-07-acme-ltd-go-engineer-remote-3"}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v", ids)
		}
	}
}

func TestIDNeverChanges(t *testing.T) {
	s := openTest(t, t.TempDir())
	a, _ := s.CreateApplication(model.Application{CompanyID: "x", Job: model.Job{Title: "Old title"}})
	b, err := s.UpdateApplication(a.ID, func(a *model.Application) error {
		a.ID = "hacked"
		a.Job.Title = "New title"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != a.ID || !b.CreatedAt.Equal(a.CreatedAt) || !b.UpdatedAt.After(a.UpdatedAt) {
		t.Fatalf("after update: %+v", b)
	}
	if _, ok := s.GetApplication("hacked"); ok {
		t.Fatal("id changed")
	}
}

func TestAtomicWriteLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	a, _ := s.CreateApplication(model.Application{CompanyID: "x", Job: model.Job{Title: "T"}})
	for i := range 5 {
		if _, err := s.UpdateApplication(a.ID, func(a *model.Application) error {
			a.Other.Notes = strings.Repeat("x", i)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if strings.HasPrefix(d.Name(), ".tmp-") {
			t.Errorf("leftover temp file %s", p)
		}
		return nil
	})
}

func TestMalformedFileReportedAndNeverOverwritten(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	s.Close()

	badApp := filepath.Join(dir, "applications", "2026-10-01-acme-broken")
	os.MkdirAll(badApp, 0o755)
	badContent := []byte("schemaVersion: 1\nid: x\njob:\n  title: [unclosed\n")
	os.WriteFile(filepath.Join(badApp, "application.yaml"), badContent, 0o644)
	os.WriteFile(filepath.Join(dir, "companies", "acme.yaml"), []byte("schemaVersion: 1\nname: Acme\nunknownKey: 1\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "companies", "future.yaml"), []byte("schemaVersion: 9\nname: Future\n"), 0o644)

	s = openTest(t, dir)
	errs := s.LoadErrors()
	if len(errs) != 3 {
		t.Fatalf("load errors = %+v", errs)
	}
	joined := ""
	for _, e := range errs {
		joined += e.Path + ": " + e.Err + "\n"
	}
	for _, want := range []string{"applications/2026-10-01-acme-broken/application.yaml", "line 3", "unknownKey", "newer version"} {
		if !strings.Contains(joined, want) {
			t.Errorf("load errors missing %q:\n%s", want, joined)
		}
	}

	// Creating records whose IDs match the broken files must not overwrite them.
	c, err := s.CreateCompany(model.Company{Name: "Acme"})
	if err != nil || c.ID != "acme-2" {
		t.Fatalf("company id = %q, err %v", c.ID, err)
	}
	if b, _ := os.ReadFile(filepath.Join(badApp, "application.yaml")); !bytes.Equal(b, badContent) {
		t.Fatal("broken application overwritten")
	}
	if _, err := s.UpdateApplication("2026-10-01-acme-broken", func(*model.Application) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update of unloaded app: %v", err)
	}
}

func TestUnknownValuesPreserved(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	a, _ := s.CreateApplication(model.Application{CompanyID: "x", Status: "on-hold", Priority: "urgent",
		Job: model.Job{Title: "T", Channel: "Meetup"}})
	s.Close()
	s = openTest(t, dir)
	_, err := s.UpdateApplication(a.ID, func(a *model.Application) error { a.Other.Notes = "n"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = openTest(t, dir)
	got, _ := s.GetApplication(a.ID)
	if got.Status != "on-hold" || got.Priority != "urgent" || got.Job.Channel != "Meetup" {
		t.Fatalf("unknown values dropped: %+v", got)
	}
}

func TestExternalEditDetected(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	a, _ := s.CreateApplication(model.Application{CompanyID: "x", Job: model.Job{Title: "T"}})
	p := filepath.Join(dir, "applications", a.ID, "application.yaml")
	b, _ := os.ReadFile(p)
	edited := bytes.Replace(b, []byte("title: T"), []byte("title: Edited by hand"), 1)
	os.WriteFile(p, edited, 0o644)
	os.Chtimes(p, time.Now().Add(time.Minute), time.Now().Add(time.Minute))

	_, err := s.UpdateApplication(a.ID, func(a *model.Application) error { a.Other.Notes = "x"; return nil })
	if !errors.Is(err, ErrExternalChange) {
		t.Fatalf("want ErrExternalChange, got %v", err)
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, edited) {
		t.Fatal("hand edit overwritten")
	}
	if err := s.Reload(); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetApplication(a.ID)
	if got.Job.Title != "Edited by hand" {
		t.Fatalf("reload didn't pick up the edit: %q", got.Job.Title)
	}
	if _, err := s.UpdateApplication(a.ID, func(a *model.Application) error { a.Other.Notes = "x"; return nil }); err != nil {
		t.Fatalf("update after reload: %v", err)
	}
}

func TestDeleteMovesToTrash(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	a, _ := s.CreateApplication(model.Application{CompanyID: "x", Job: model.Job{Title: "T"}})
	if err := s.DeleteApplication(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.GetApplication(a.ID); ok {
		t.Fatal("still in memory")
	}
	if _, err := os.Stat(filepath.Join(dir, "applications", a.ID)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("folder still in applications/")
	}
	trash, _ := os.ReadDir(filepath.Join(dir, ".trash"))
	if len(trash) != 1 || !strings.HasSuffix(trash[0].Name(), a.ID) {
		t.Fatalf("trash = %v", trash)
	}
	if _, err := os.Stat(filepath.Join(dir, ".trash", trash[0].Name(), "application.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestFiles(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	a, _ := s.CreateApplication(model.Application{CompanyID: "x", Job: model.Job{Title: "T"}})

	name, err := s.SaveFile(a.ID, `C:\Users\me\My Cover Letter (final).pdf`, strings.NewReader("pdf"))
	if err != nil || name != "My-Cover-Letter-final.pdf" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	name2, _ := s.SaveFile(a.ID, "My Cover Letter (final).pdf", strings.NewReader("pdf2"))
	if name2 != "My-Cover-Letter-final-2.pdf" {
		t.Fatalf("second name = %q", name2)
	}
	f, err := s.OpenFile(a.ID, name2)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(f)
	f.Close()
	if string(b) != "pdf2" {
		t.Fatalf("content = %q", b)
	}
	files, _ := s.ListFiles(a.ID)
	if len(files) != 2 {
		t.Fatalf("files = %+v", files)
	}

	if err := s.ReplaceFile(a.ID, "posting.md", []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceFile(a.ID, "posting.md", []byte("v2")); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFile(a.ID, name); err != nil {
		t.Fatal(err)
	}
	trash, _ := os.ReadDir(filepath.Join(dir, ".trash"))
	if len(trash) != 2 { // old posting.md + deleted cover letter
		t.Fatalf("trash = %v", trash)
	}

	if _, err := s.SaveCV("CV backend v3.pdf", strings.NewReader("cv")); err != nil {
		t.Fatal(err)
	}
	cvs, _ := s.ListCVs()
	if len(cvs) != 1 || cvs[0].Name != "CV-backend-v3.pdf" {
		t.Fatalf("cvs = %+v", cvs)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	a, _ := s.CreateApplication(model.Application{CompanyID: "x", Job: model.Job{Title: "T"}})
	os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("secret"), 0o644)

	for _, name := range []string{"../../../secret.txt", `..\..\..\secret.txt`, "..", ".", "", "a/b", ".tmp-x"} {
		if f, err := s.OpenFile(a.ID, name); err == nil {
			f.Close()
			t.Errorf("OpenFile(%q) succeeded", name)
		}
		if err := s.DeleteFile(a.ID, name); err == nil {
			t.Errorf("DeleteFile(%q) succeeded", name)
		}
	}
	if _, err := s.OpenFile("../..", "secret.txt"); err == nil {
		t.Error("OpenFile with traversal app id succeeded")
	}
	if f, err := s.OpenCV("../secret.txt"); err == nil {
		f.Close()
		t.Error("OpenCV traversal succeeded")
	}
	name, _ := s.SaveFile(a.ID, "../../secret.txt", strings.NewReader("x"))
	if name != "secret.txt" {
		t.Errorf("SaveFile name = %q", name)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "secret.txt")); string(b) != "secret" {
		t.Error("file outside the app folder was overwritten")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := map[string]string{
		"cv.pdf":               "cv.pdf",
		"../../etc/passwd":     "passwd",
		`C:\x\Résumé 2026.pdf`: "Resume-2026.pdf",
		"...hidden":            "file.hidden",
		"简历.pdf":               "file.pdf",
		"CON.txt":              "file-CON.txt",
		"a  b--c.":             "a-b-c",
	}
	for in, want := range tests {
		if got := SanitizeFilename(in); got != want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBackupZip(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	a, _ := s.CreateApplication(model.Application{CompanyID: "x", Job: model.Job{Title: "T"}})
	s.SaveFile(a.ID, "note.txt", strings.NewReader("hi"))
	var buf bytes.Buffer
	if err := s.WriteBackup(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	for _, want := range []string{"config.yaml", "applications/" + a.ID + "/application.yaml", "applications/" + a.ID + "/files/note.txt"} {
		if !names[want] {
			t.Errorf("zip missing %s; has %v", want, names)
		}
	}
	if names[".lock"] {
		t.Error("zip contains .lock")
	}
}
