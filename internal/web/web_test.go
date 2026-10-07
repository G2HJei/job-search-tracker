package web

import (
	"archive/zip"
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/G2HJei/job-search-tracker/internal/demo"
	"github.com/G2HJei/job-search-tracker/internal/store"
	webassets "github.com/G2HJei/job-search-tracker/web"
)

var bst = time.FixedZone("BST", 3600)

// fixedNow is the demo data's reference day.
func fixedNow() time.Time { return time.Date(2026, 10, 7, 10, 0, 0, 0, bst) }

type testApp struct {
	t     *testing.T
	dir   string
	store *store.Store
	srv   *httptest.Server
	c     *http.Client
}

// newTestApp starts a server on a fresh data directory, optionally seeded
// with the demo data.
func newTestApp(t *testing.T, seed bool) *testApp {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "data")
	if seed {
		if err := demo.CopyTo(dir, demo.Today); err != nil {
			t.Fatal(err)
		}
	}
	quiet := slog.New(slog.DiscardHandler)
	st, err := store.Open(dir, store.Options{Now: fixedNow, Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Store: st, Static: webassets.Static(), Version: "test", Addr: "127.0.0.1:0", Logger: quiet, Now: fixedNow})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(func() {
		srv.Close()
		st.Close()
	})
	return &testApp{t: t, dir: dir, store: st, srv: srv, c: &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

type resp struct {
	*http.Response
	body string
}

func (a *testApp) do(req *http.Request) resp {
	a.t.Helper()
	res, err := a.c.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return resp{res, string(b)}
}

func (a *testApp) get(path string, hx bool) resp {
	a.t.Helper()
	req, _ := http.NewRequest("GET", a.srv.URL+path, nil)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	return a.do(req)
}

func (a *testApp) post(path string, form url.Values, hx bool) resp {
	a.t.Helper()
	req, _ := http.NewRequest("POST", a.srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	return a.do(req)
}

func (a *testApp) readFile(rel string) string {
	a.t.Helper()
	b, err := os.ReadFile(filepath.Join(a.dir, rel))
	if err != nil {
		a.t.Fatal(err)
	}
	return string(b)
}

func expect(t *testing.T, r resp, status int, contains ...string) {
	t.Helper()
	if r.StatusCode != status {
		t.Fatalf("%s %s: status %d, want %d\n%s", r.Request.Method, r.Request.URL.Path, r.StatusCode, status, r.body)
	}
	for _, s := range contains {
		if !strings.Contains(r.body, s) {
			t.Fatalf("%s %s: body does not contain %q\n%s", r.Request.Method, r.Request.URL.Path, s, r.body)
		}
	}
}

var revRe = regexp.MustCompile(`name="rev" value="([^"]*)"`)

func rev(t *testing.T, body string) string {
	t.Helper()
	m := revRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no rev in form:\n%s", body)
	}
	return m[1]
}

const appID = "2026-10-07-acme-backend-engineer"

func (a *testApp) create() {
	a.t.Helper()
	r := a.post("/applications", url.Values{
		"company": {"Acme"}, "title": {"Backend Engineer"}, "postingUrl": {"https://acme.example/jobs/1"},
		"priority": {"high"}, "status": {"applied"},
	}, false)
	expect(a.t, r, http.StatusSeeOther)
	if loc := r.Header.Get("Location"); loc != "/applications/"+appID {
		a.t.Fatalf("Location = %q", loc)
	}
}

func TestCreateEditEverySection(t *testing.T) {
	a := newTestApp(t, false)
	a.create()
	expect(t, a.get("/applications/"+appID, false), 200, "Backend Engineer", "Acme", `id="section-process"`)
	yaml := a.readFile("applications/" + appID + "/application.yaml")
	for _, want := range []string{"status: applied", "priority: high", "companyId: acme", "dateApplied: 2026-10-07"} {
		if !strings.Contains(yaml, want) {
			t.Fatalf("new application.yaml missing %q:\n%s", want, yaml)
		}
	}

	sections := []struct {
		name string
		form url.Values
		show []string // expected in the view partial
		yaml []string // expected in the saved file
	}{
		{"company", url.Values{"name": {"Acme Corp"}, "industry": {"Fintech"}, "size": {"201-1000"},
			"hq": {"London"}, "website": {"https://acme.example"}, "notes": {"Good **blog**"}},
			[]string{"Acme Corp", "Fintech", "<strong>blog</strong>"}, nil},
		{"job", url.Values{"title": {"Senior Backend Engineer"}, "postingUrl": {"https://acme.example/jobs/1"},
			"postingCopy": {"posting.pdf"}, "jobId": {"R-1"}, "department": {"Payments"}, "location": {"London"},
			"keyRequirements": {"- Go\r\n- Postgres  \r\n"}, "fit": {"4"}, "interest": {"5"}, "channel": {"Referral"},
			"cvVersion": {"cv-v3.pdf"}, "material": {"cover-letter.pdf", "", "https://github.com/me"},
			"advertisedSalary_min": {"80,000"}, "advertisedSalary_max": {"95000"}, "advertisedSalary_currency": {"gbp"},
			"advertisedSalary_period": {"year"}, "advertisedSalary_note": {""}},
			[]string{"Senior Backend Engineer", "★★★★☆", "£80,000–95,000 / year", "<li>Postgres</li>"},
			[]string{"title: Senior Backend Engineer", "keyRequirements: |\n    - Go\n    - Postgres\n",
				"advertisedSalary: {min: 80000, max: 95000, currency: GBP, period: year}",
				"extraMaterials:\n    - cover-letter.pdf\n    - https://github.com/me\n"}},
		{"contacts", url.Values{
			"referrer_name": {"Jane"}, "referrer_role": {"Staff Eng"}, "referrer_email": {"jane@example.com"},
			"referrer_phone": {""}, "referrer_linkedin": {""}, "referrer_notes": {""},
			"recruiter_name": {"Tom", "", "Ann"}, "recruiter_role": {"Talent", "", ""}, "recruiter_email": {"", "", "ann@x.example"},
			"recruiter_phone": {"", "", ""}, "recruiter_linkedin": {"https://linkedin.com/in/tom", "", ""}, "recruiter_notes": {"", "", ""},
			"hm_name": {""}, "hm_role": {""}, "hm_email": {""}, "hm_phone": {""}, "hm_linkedin": {""}, "hm_notes": {""},
			"interviewer_name": {"Alex"}, "interviewer_role": {""}, "interviewer_email": {""}, "interviewer_phone": {""},
			"interviewer_linkedin": {""}, "interviewer_notes": {""}},
			[]string{"Jane", "Tom", "Ann", "Alex", "Hiring manager"},
			[]string{"recruiters:\n    - name: Tom\n", "    - name: Ann\n", "interviewers:\n    - name: Alex\n"}},
		{"process", url.Values{"nextStep": {"Tech interview"}, "nextStepDate": {"2026-10-14"},
			"lastContactDate": {"2026-10-07"}, "dateApplied": {"2026-10-01"}, "screeningDate": {"2026-10-09T10:00"},
			"interview_date": {"2026-10-14T14:00", "", "2026-10-20T00:00"}, "interview_type": {"Technical", "", "Final"},
			"interview_interviewers": {"Alex, Sam", "", ""}, "interview_notes": {"", "", ""},
			"followUpDate": {"2026-10-16"}, "offerDate": {""}, "offerResponseDeadline": {""},
			"qa_round": {"0", "1"}, "qa_question": {"Why leave?", "Design a cache"}, "qa_answer": {"Growth", ""},
			"takeHomeDetails": {"Build a thing"}, "takeHomeDeadline": {"2026-10-12T17:00"},
			"decision": {"Pending"}, "rejectionFeedback": {""}},
			[]string{"Tech interview", "Interview 1", "Interview 2", "Why leave?", "Take-home deadline"},
			[]string{"nextStepDate: 2026-10-14", "screeningDate: 2026-10-09T10:00",
				"- date: 2026-10-14T14:00\n      type: Technical\n      interviewers: [Alex, Sam]",
				"- date: 2026-10-20\n      type: Final", "- round: 1\n      question: Design a cache",
				"deadline: 2026-10-12T17:00"}},
		{"compensation", url.Values{"asked_min": {"92000"}, "asked_max": {""}, "asked_currency": {"GBP"}, "asked_period": {"year"},
			"asked_note": {""}, "offered_min": {""}, "offered_max": {""}, "offered_currency": {"GBP"}, "offered_period": {"year"},
			"offered_note": {""}, "contractType": {"Permanent"}, "bonus": {"10%"}, "equity": {""}, "benefits": {"- Pension"},
			"noticePeriod": {"1 month"}, "startDate": {"2026-11-02"}},
			[]string{"£92,000+ / year", "Permanent", "1 month", "2 Nov 2026"},
			[]string{"asked: {min: 92000, currency: GBP, period: year}", "startDate: 2026-11-02"}},
		{"other", url.Values{"lessonsLearned": {"Ask about on-call"}, "notes": {"Line one\r\nLine two"}},
			[]string{"Ask about on-call", "Line one<br>"}, []string{"notes: |\n    Line one\n    Line two\n"}},
	}
	for _, s := range sections {
		t.Run(s.name, func(t *testing.T) {
			edit := a.get("/applications/"+appID+"/sections/"+s.name+"/edit", true)
			expect(t, edit, 200, "<form", `hx-post="/applications/`+appID+`/sections/`+s.name+`"`)
			s.form.Set("rev", rev(t, edit.body))
			r := a.post("/applications/"+appID+"/sections/"+s.name, s.form, true)
			expect(t, r, 200, append([]string{`id="section-` + s.name + `"`}, s.show...)...)
			if r.Header.Get("HX-Trigger") != `{"toast":"Saved"}` {
				t.Errorf("HX-Trigger = %q", r.Header.Get("HX-Trigger"))
			}
			yaml := a.readFile("applications/" + appID + "/application.yaml")
			for _, want := range s.yaml {
				if !strings.Contains(yaml, want) {
					t.Errorf("application.yaml missing %q:\n%s", want, yaml)
				}
			}
		})
	}
	if c := a.readFile("companies/acme.yaml"); !strings.Contains(c, "name: Acme Corp") || !strings.Contains(c, "id: acme") {
		t.Errorf("company file:\n%s", c)
	}

	// Data survives a restart.
	a.store.Close()
	st, err := store.Open(a.dir, store.Options{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	got, ok := st.GetApplication(appID)
	if !ok || got.Job.Title != "Senior Backend Engineer" || len(got.Contacts.Recruiters) != 2 ||
		len(got.Process.Interviews) != 2 || got.Process.Interviews[1].Date.HasTime() || got.Compensation.Asked.Min != 92000 {
		t.Fatalf("after restart: %+v", got)
	}
}

func TestValidationErrorsRerenderForm(t *testing.T) {
	a := newTestApp(t, false)
	a.create()
	r := a.post("/applications/"+appID+"/sections/job", url.Values{
		"title": {""}, "postingUrl": {"javascript:alert(1)"}, "fit": {"4"},
		"advertisedSalary_min": {"100"}, "advertisedSalary_max": {"50"},
	}, true)
	expect(t, r, http.StatusUnprocessableEntity, "Job title is required", "http:// or https://",
		"Minimum is higher than maximum", `aria-invalid="true"`, `value="4" checked`)

	r = a.post("/applications/"+appID+"/sections/process", url.Values{"nextStepDate": {"14/10/2026"}}, true)
	expect(t, r, http.StatusUnprocessableEntity, "Use a date like 2026-10-14")

	// Without htmx the whole page comes back with the section in edit mode.
	r = a.post("/applications/"+appID+"/sections/job", url.Values{"title": {""}}, false)
	expect(t, r, http.StatusUnprocessableEntity, "<!doctype html>", "Job title is required", `id="section-process"`)

	if y := a.readFile("applications/" + appID + "/application.yaml"); !strings.Contains(y, "title: Backend Engineer") {
		t.Fatalf("invalid save was written:\n%s", y)
	}

	r = a.post("/applications", url.Values{"company": {""}, "title": {""}, "priority": {"high"}, "status": {"nope"}}, false)
	expect(t, r, http.StatusUnprocessableEntity, "Company is required", "Job title is required", "Pick a status")
}

func TestNoJSFlow(t *testing.T) {
	a := newTestApp(t, false)
	a.create()
	expect(t, a.get("/applications/"+appID+"/sections/other/edit", false), 200, "<!doctype html>", `name="lessonsLearned"`)
	r := a.post("/applications/"+appID+"/sections/other", url.Values{"notes": {"plain post"}}, false)
	expect(t, r, http.StatusSeeOther)
	if r.Header.Get("Location") != "/applications/"+appID+"#section-other" {
		t.Fatalf("Location = %q", r.Header.Get("Location"))
	}
	r = a.post("/applications/"+appID+"/status", url.Values{"status": {"screening"}, "return": {"/board"}}, false)
	expect(t, r, http.StatusSeeOther)
	if r.Header.Get("Location") != "/board" {
		t.Fatalf("Location = %q", r.Header.Get("Location"))
	}
	r = a.post("/applications/"+appID+"/status", url.Values{"status": {"screening"}, "return": {"//evil.example"}}, false)
	if r.Header.Get("Location") != "/applications/"+appID {
		t.Fatalf("open redirect: Location = %q", r.Header.Get("Location"))
	}
}

func TestConflictDetected(t *testing.T) {
	a := newTestApp(t, false)
	a.create()
	edit := a.get("/applications/"+appID+"/sections/other/edit", true)
	oldRev := rev(t, edit.body)

	// Another tab saves the same section first.
	expect(t, a.post("/applications/"+appID+"/sections/other", url.Values{"rev": {oldRev}, "notes": {"from tab 1"}}, true), 200)
	r := a.post("/applications/"+appID+"/sections/other", url.Values{"rev": {oldRev}, "notes": {"from tab 2"}}, true)
	expect(t, r, http.StatusConflict, "changed elsewhere", "from tab 2")
	if y := a.readFile("applications/" + appID + "/application.yaml"); !strings.Contains(y, "from tab 1") {
		t.Fatalf("conflicting save overwrote:\n%s", y)
	}
	// Saving again from the conflict form overwrites deliberately.
	expect(t, a.post("/applications/"+appID+"/sections/other", url.Values{"rev": {rev(t, r.body)}, "notes": {"from tab 2"}}, true), 200)

	// Changes to other sections (like a status change) don't conflict.
	edit = a.get("/applications/"+appID+"/sections/job/edit", true)
	expect(t, a.post("/applications/"+appID+"/status", url.Values{"status": {"offer"}, "ctx": {"detail"}}, true), 200)
	expect(t, a.post("/applications/"+appID+"/sections/job", url.Values{"rev": {rev(t, edit.body)}, "title": {"Still fine"}}, true), 200)

	// A hand edit on disk is detected.
	p := filepath.Join(a.dir, "applications", appID, "application.yaml")
	b, _ := os.ReadFile(p)
	os.WriteFile(p, bytes.Replace(b, []byte("Still fine"), []byte("Edited by hand"), 1), 0o644)
	os.Chtimes(p, time.Now().Add(time.Minute), time.Now().Add(time.Minute))
	expect(t, a.post("/applications/"+appID+"/sections/other", url.Values{"notes": {"x"}}, true), http.StatusConflict, "changed on disk")
	expect(t, a.post("/settings/reload", nil, false), http.StatusSeeOther)
	expect(t, a.get("/applications/"+appID, false), 200, "Edited by hand")
}

func TestQuickActions(t *testing.T) {
	a := newTestApp(t, false)
	a.create()
	r := a.post("/applications/"+appID+"/status", url.Values{"status": {"interviewing"}, "ctx": {"detail"}}, true)
	expect(t, r, 200, `id="app-header"`, `<option value="interviewing" selected>`)
	if !strings.Contains(r.Header.Get("HX-Trigger"), "Status: Interviewing") {
		t.Errorf("HX-Trigger = %q", r.Header.Get("HX-Trigger"))
	}
	expect(t, a.post("/applications/"+appID+"/status", url.Values{"status": {"rejected"}, "ctx": {"board"}}, true), http.StatusNoContent)
	expect(t, a.post("/applications/"+appID+"/status", url.Values{"status": {"made-up"}}, true), http.StatusBadRequest)
	expect(t, a.post("/applications/"+appID+"/priority", url.Values{"priority": {"low"}, "ctx": {"detail"}}, true), 200)

	// "Contacted today" changes the process section, so it is refreshed out of band.
	r = a.post("/applications/"+appID+"/touch", url.Values{"ctx": {"detail"}}, true)
	expect(t, r, 200, `id="section-process"`, `hx-swap-oob="true"`, "7 Oct 2026")
	y := a.readFile("applications/" + appID + "/application.yaml")
	for _, want := range []string{"status: rejected", "priority: low", "lastContactDate: 2026-10-07"} {
		if !strings.Contains(y, want) {
			t.Errorf("missing %q:\n%s", want, y)
		}
	}

	// A decision offers the matching status; the suggestion button applies it.
	expect(t, a.post("/applications/"+appID+"/sections/process", url.Values{"decision": {"Offer accepted"}}, true), 200, "Set status to Accepted")
	r = a.post("/applications/"+appID+"/status", url.Values{"status": {"accepted"}, "ctx": {"suggest"}}, true)
	expect(t, r, 200, `id="section-process"`, `id="app-header"`)
	if strings.Contains(r.body, "Set status to") {
		t.Error("suggestion still shown after applying it")
	}

	expect(t, a.post("/applications/"+appID+"/delete", nil, false), http.StatusSeeOther)
	expect(t, a.get("/applications/"+appID, false), http.StatusNotFound)
	if entries, _ := os.ReadDir(filepath.Join(a.dir, ".trash")); len(entries) != 1 {
		t.Fatalf("trash = %v", entries)
	}
}

func multipartBody(t *testing.T, fields map[string]string, filename, content string) (*bytes.Buffer, string) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	fw, _ := mw.CreateFormFile("file", filename)
	fw.Write([]byte(content))
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func (a *testApp) upload(path string, fields map[string]string, filename, content string) resp {
	body, ct := multipartBody(a.t, fields, filename, content)
	req, _ := http.NewRequest("POST", a.srv.URL+path, body)
	req.Header.Set("Content-Type", ct)
	return a.do(req)
}

func TestFiles(t *testing.T) {
	a := newTestApp(t, false)
	a.create()
	base := "/applications/" + appID
	expect(t, a.upload(base+"/files", map[string]string{"purpose": "posting"}, "Job Ad.html", "<script>alert(1)</script>"), http.StatusSeeOther)
	expect(t, a.upload(base+"/files", map[string]string{"purpose": "material"}, "cover letter.pdf", "%PDF-1.4"), http.StatusSeeOther)
	expect(t, a.upload(base+"/files", nil, "data.bin", "\x00\x01"), http.StatusSeeOther)

	y := a.readFile("applications/" + appID + "/application.yaml")
	if !strings.Contains(y, "postingCopy: Job-Ad.html") || !strings.Contains(y, "- cover-letter.pdf") {
		t.Fatalf("upload purposes not recorded:\n%s", y)
	}
	expect(t, a.get(base, false), 200, "Job-Ad.html", "posting copy", "cover-letter.pdf", "sent")

	r := a.get(base+"/files/Job-Ad.html", false)
	expect(t, r, 200, "<script>")
	if r.Header.Get("Content-Security-Policy") != "sandbox" {
		t.Errorf("HTML served without sandbox CSP: %q", r.Header.Get("Content-Security-Policy"))
	}
	r = a.get(base+"/files/cover-letter.pdf", false)
	if r.Header.Get("Content-Type") != "application/pdf" || !strings.HasPrefix(r.Header.Get("Content-Disposition"), "inline") {
		t.Errorf("pdf headers: %v", r.Header)
	}
	r = a.get(base+"/files/data.bin", false)
	if !strings.HasPrefix(r.Header.Get("Content-Disposition"), "attachment") {
		t.Errorf("binary not served as attachment: %v", r.Header)
	}

	for _, p := range []string{"/files/..%2F..%2Fconfig.yaml", "/files/..%5C..%5Cconfig.yaml", "/files/.lock"} {
		if r := a.get(base+p, false); r.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status %d", p, r.StatusCode)
		}
	}

	expect(t, a.post(base+"/posting-text", url.Values{"text": {"# Senior Engineer\r\nWe need **you**."}}, false), http.StatusSeeOther)
	expect(t, a.get(base, false), 200, "Saved posting (posting.md)", "<strong>you</strong>")
	expect(t, a.post(base+"/files/data.bin/delete", nil, false), http.StatusSeeOther)
	expect(t, a.get(base+"/files/data.bin", false), http.StatusNotFound)

	// CV library and reverse lookup.
	expect(t, a.upload("/cv", nil, "CV Backend v3.pdf", "%PDF"), http.StatusSeeOther)
	expect(t, a.post(base+"/sections/job", url.Values{"title": {"Backend Engineer"}, "cvVersion": {"CV-Backend-v3.pdf"}}, true), 200, `href="/cv/CV-Backend-v3.pdf"`)
	expect(t, a.get("/cv", false), 200, "CV-Backend-v3.pdf", "Acme · Backend Engineer")
	expect(t, a.get("/cv/CV-Backend-v3.pdf", false), 200, "%PDF")
}

func TestUploadTooLarge(t *testing.T) {
	a := newTestApp(t, false)
	a.create()
	r := a.upload("/applications/"+appID+"/files", nil, "big.bin", strings.Repeat("x", maxUpload+2<<20))
	expect(t, r, http.StatusBadRequest, "larger than 25 MB")
}

func TestSecurity(t *testing.T) {
	a := newTestApp(t, false)
	a.create()

	req, _ := http.NewRequest("POST", a.srv.URL+"/applications/"+appID+"/delete", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if r := a.do(req); r.StatusCode != http.StatusForbidden {
		t.Errorf("cross-site POST: status %d", r.StatusCode)
	}
	req, _ = http.NewRequest("POST", a.srv.URL+"/applications/"+appID+"/delete", nil)
	req.Header.Set("Origin", "https://evil.example")
	if r := a.do(req); r.StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin POST: status %d", r.StatusCode)
	}
	if _, ok := a.store.GetApplication(appID); !ok {
		t.Fatal("cross-origin request deleted the application")
	}

	req, _ = http.NewRequest("GET", a.srv.URL+"/", nil)
	req.Host = "evil.example"
	if r := a.do(req); r.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("bad Host: status %d", r.StatusCode)
	}
	req, _ = http.NewRequest("GET", a.srv.URL+"/", nil)
	req.Host = "localhost:8765"
	if r := a.do(req); r.StatusCode != http.StatusOK {
		t.Errorf("localhost Host: status %d", r.StatusCode)
	}

	r := a.get("/", false)
	if csp := r.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP = %q", csp)
	}
	if strings.Contains(r.body, "<script>") || strings.Contains(r.body, " style=") {
		t.Error("page has inline script or style, which the CSP blocks")
	}

	// User content is escaped and Markdown drops raw HTML and javascript: links.
	expect(t, a.post("/applications/"+appID+"/sections/other", url.Values{
		"notes": {"<img src=x onerror=alert(1)> [x](javascript:alert(1))"}}, true), 200)
	r = a.get("/applications/"+appID, false)
	if strings.Contains(r.body, "<img src=x") || strings.Contains(r.body, `href="javascript:`) {
		t.Errorf("unsafe HTML rendered:\n%s", r.body)
	}
}

func TestHostAllowlistAllInterfaces(t *testing.T) {
	s := New(Options{Addr: "0.0.0.0:8765"})
	h := s.checkHost(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for host, want := range map[string]int{"192.168.1.20:8765": 200, "[::1]:8765": 200, "evil.example:8765": 421} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %s: %d, want %d", host, rec.Code, want)
		}
	}
}

func TestPagesWithDemoData(t *testing.T) {
	a := newTestApp(t, true)
	expect(t, a.get("/", false), 200, "Overdue <span class=\"count\">3</span>", "Needs follow-up <span class=\"count\">2</span>",
		"Harbourline Logistics · Senior Software Engineer, Routing", "Interview 3 · Final")
	expect(t, a.get("/board", false), 200, "data-sortable", "Sortable.min.js")
	expect(t, a.get("/calendar", false), 200, "November 2026", "Start date")
	expect(t, a.get("/companies", false), 200, "Larkspur Payments", "Mossgate")
	expect(t, a.get("/companies/larkspur-payments", false), 200, "Payments Core", "Fraud Platform")
	expect(t, a.get("/cv", false), 200, "cv-backend-v3.pdf", "Quillfeather Labs · Platform Engineer")
	expect(t, a.get("/settings", false), 200, "All files loaded", "defaultCurrency")
	// The deliberately unknown channel is kept and flagged.
	expect(t, a.get("/applications/2026-08-10-fernhollow-energy-go-developer-6-month-contract", false), 200,
		`class="unknown-value" title="Not in config.yaml">Meetup`)

	// Active applications by default; closed ones with the toggle.
	r := a.get("/applications", false)
	expect(t, r, 200, "6 of 10 applications", "<!doctype html>")
	expect(t, a.get("/applications?closed=1", false), 200, "10 of 10 applications")
	expect(t, a.get("/applications?status=rejected", false), 200, "1 of 10 applications", "Fraud Platform")

	// htmx search returns just the results.
	req, _ := http.NewRequest("GET", a.srv.URL+"/applications?q=quillfeather+platform&closed=1", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "results")
	r = a.do(req)
	expect(t, r, 200, "1 of 10 applications", "Platform Engineer", `name="sort"`)
	if strings.Contains(r.body, "<html") {
		t.Error("htmx search returned a full page")
	}
	// Search covers contacts.
	expect(t, a.get("/applications?q=grace+adeyemi", false), 200, "Payments Core")

	// Sorting by company puts Bramblewood first, descending puts Quillfeather first.
	r = a.get("/applications?sort=company&dir=asc", false)
	if i, j := strings.Index(r.body, "Bramblewood"), strings.Index(r.body, "Quillfeather"); i < 0 || j < 0 || i > j {
		t.Error("sort by company asc")
	}
	r = a.get("/applications?sort=company&dir=desc", false)
	if i, j := strings.Index(r.body, "Bramblewood"), strings.Index(r.body, "Quillfeather"); i < j {
		t.Error("sort by company desc")
	}
}

func TestExports(t *testing.T) {
	a := newTestApp(t, true)

	r := a.get("/calendar.ics", false)
	expect(t, r, 200, "BEGIN:VCALENDAR\r\n", "END:VCALENDAR\r\n", "DTSTART:20261013T140000", "DTSTART;VALUE=DATE:20261102")
	for _, l := range strings.Split(r.body, "\r\n") {
		if len(l) > 75 {
			t.Errorf("unfolded line: %q", l)
		}
	}
	if strings.Contains(r.body, "20261012") { // the withdrawn application's cancelled interview
		t.Error("closed application's event exported")
	}

	r = a.get("/export.csv", false)
	expect(t, r, 200, "\uFEFFid,createdAt,updatedAt,priority,status,company")
	if n := strings.Count(r.body, "\n2026-"); n < 10 {
		t.Errorf("csv rows = %d", n)
	}

	r = a.get("/backup.zip", false)
	zr, err := zip.NewReader(strings.NewReader(r.body), int64(len(r.body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) < 20 {
		t.Errorf("backup has %d files", len(zr.File))
	}
}

func TestBlankRows(t *testing.T) {
	a := newTestApp(t, false)
	expect(t, a.get("/partials/row/contact?prefix=recruiter", true), 200, `name="recruiter_name"`, "data-row")
	expect(t, a.get("/partials/row/interview", true), 200, `name="interview_date"`, "Phone screen")
	expect(t, a.get("/partials/row/qa?rounds=5", true), 200, "Interview 5", `name="qa_question"`)
	expect(t, a.get("/partials/row/material", true), 200, `name="material"`)
	expect(t, a.get("/partials/row/contact?prefix=evil", true), http.StatusBadRequest)
	expect(t, a.get("/partials/row/nope", true), http.StatusNotFound)
}

func TestStaticAndHealth(t *testing.T) {
	a := newTestApp(t, false)
	expect(t, a.get("/healthz", false), 200, "ok")
	expect(t, a.get("/static/vendor/htmx.min.js", false), 200, "htmx")
	expect(t, a.get("/static/css/app.css", false), 200, ".pill")
	expect(t, a.get("/nope", false), 404, "There is nothing at /nope")
}
