package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"

	"github.com/G2HJei/job-search-tracker/internal/store"
	"github.com/G2HJei/job-search-tracker/internal/views"
)

// isHX reports whether htmx made the request and expects a partial. History
// restores ask for the full page.
func isHX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-History-Restore-Request") != "true"
}

// page returns the layout data for a full page.
func (s *Server) page(title, nav string) views.Page {
	return views.Page{
		Title:      title,
		Nav:        nav,
		Version:    s.assetVer,
		LoadErrors: len(s.store.LoadErrors()),
		Warnings:   s.store.Warnings(),
	}
}

// render writes a component. It renders into a buffer first so a template
// error becomes a clean 500 instead of a half-written page.
func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, components ...templ.Component) {
	var buf bytes.Buffer
	for _, c := range components {
		if err := c.Render(r.Context(), &buf); err != nil {
			s.log.Error("render", "path", r.URL.Path, "err", err)
			http.Error(w, "Could not render the page: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Add("Vary", "HX-Request")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}

// toast asks the page to show a short notification.
func toast(w http.ResponseWriter, msg string) {
	b, _ := json.Marshal(map[string]string{"toast": msg})
	w.Header().Set("HX-Trigger", string(b))
}

// fail shows an error: plain text for htmx (shown as a toast), a full error
// page otherwise.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, status int, msg string) {
	if isHX(r) {
		http.Error(w, msg, status)
		return
	}
	s.render(w, r, status, views.ErrorPage(s.page(http.StatusText(status), ""), status, msg))
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.fail(w, r, http.StatusNotFound, "There is nothing at "+r.URL.Path+".")
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	s.fail(w, r, http.StatusInternalServerError, "Something went wrong: "+err.Error())
}

// storeError maps store errors to HTTP responses.
func (s *Server) storeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.fail(w, r, http.StatusNotFound, "Not found. It may have been deleted, or a reload from disk may be needed.")
	case errors.Is(err, store.ErrConflict), errors.Is(err, store.ErrExternalChange):
		s.fail(w, r, http.StatusConflict, capitalize(err.Error())+".")
	default:
		s.serverError(w, r, err)
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func redirect(w http.ResponseWriter, r *http.Request, to string) {
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// localPath reports whether p is a path on this site (not "//host/…").
func localPath(p string) bool {
	return strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//") && !strings.HasPrefix(p, `/\`)
}

// returnTo picks where to go after a form post: the "return" field, else the
// referring page on this site, else fallback.
func returnTo(r *http.Request, fallback string) string {
	if p := r.PostFormValue("return"); localPath(p) {
		return p
	}
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Host == r.Host && localPath(ref.Path) {
		if ref.RawQuery != "" {
			return ref.Path + "?" + ref.RawQuery
		}
		return ref.Path
	}
	return fallback
}
