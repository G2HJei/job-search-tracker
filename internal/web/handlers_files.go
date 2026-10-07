package web

import (
	"errors"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/store"
	"github.com/G2HJei/job-search-tracker/internal/views"
)

// readUpload reads the "file" field of a multipart form, capped at maxUpload.
func readUpload(w http.ResponseWriter, r *http.Request) (multipart.File, *multipart.FileHeader, string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+1<<20) // allow for multipart overhead
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return nil, nil, "The file is larger than 25 MB."
		}
		return nil, nil, "Could not read the upload: " + err.Error()
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		return nil, nil, "Choose a file to upload."
	}
	if hdr.Size > maxUpload {
		file.Close()
		return nil, nil, "The file is larger than 25 MB."
	}
	return file, hdr, ""
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.store.GetApplication(id); !ok {
		s.notFound(w, r)
		return
	}
	file, hdr, msg := readUpload(w, r)
	if msg != "" {
		s.fail(w, r, http.StatusBadRequest, msg)
		return
	}
	defer file.Close()
	defer r.MultipartForm.RemoveAll()
	name, err := s.store.SaveFile(id, hdr.Filename, file)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	purpose := r.PostFormValue("purpose")
	if purpose == "posting" || purpose == "material" {
		_, err = s.store.UpdateApplication(id, func(a *model.Application) error {
			if purpose == "posting" {
				a.Job.PostingCopy = name
			} else if !slices.Contains(a.Job.ExtraMaterials, name) {
				a.Job.ExtraMaterials = append(a.Job.ExtraMaterials, name)
			}
			return nil
		})
		if err != nil {
			s.storeError(w, r, err)
			return
		}
	}
	redirect(w, r, "/applications/"+id+"#attachments")
}

func (s *Server) savePostingText(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, err := newForm(r)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	text := f.text("text")
	if text == "" {
		redirect(w, r, "/applications/"+id+"#attachments")
		return
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	const name = "posting.md"
	if err := s.store.ReplaceFile(id, name, []byte(text)); err != nil {
		s.storeError(w, r, err)
		return
	}
	if _, err := s.store.UpdateApplication(id, func(a *model.Application) error {
		a.Job.PostingCopy = name
		return nil
	}); err != nil {
		s.storeError(w, r, err)
		return
	}
	redirect(w, r, "/applications/"+id+"#attachments")
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request) {
	f, err := s.store.OpenFile(r.PathValue("id"), r.PathValue("name"))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	defer f.Close()
	serveFile(w, r, f, r.PathValue("name"))
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteFile(id, r.PathValue("name")); err != nil {
		s.storeError(w, r, err)
		return
	}
	redirect(w, r, "/applications/"+id+"#attachments")
}

var inlineTypes = map[string]string{
	".pdf":  "application/pdf",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".txt":  "text/plain; charset=utf-8",
	".md":   "text/plain; charset=utf-8",
}

// serveFile sends a stored file. PDFs, images and text open in the browser;
// saved HTML pages open sandboxed so their scripts can't run on the app's
// origin; everything else is downloaded.
func serveFile(w http.ResponseWriter, r *http.Request, f *os.File, name string) {
	info, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h := w.Header()
	ext := strings.ToLower(path.Ext(name))
	disposition := "attachment"
	switch {
	case inlineTypes[ext] != "":
		h.Set("Content-Type", inlineTypes[ext])
		disposition = "inline"
	case ext == ".html" || ext == ".htm":
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Content-Security-Policy", "sandbox")
		disposition = "inline"
	default:
		h.Set("Content-Type", "application/octet-stream")
	}
	h.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	h.Set("Cache-Control", "no-store")
	http.ServeContent(w, r, name, info.ModTime(), f)
}

func (s *Server) listCVs(w http.ResponseWriter, r *http.Request) {
	s.renderCVs(w, r, http.StatusOK, "")
}

func (s *Server) renderCVs(w http.ResponseWriter, r *http.Request, status int, msg string) {
	files, err := s.store.ListCVs()
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	rows := s.rows(s.store.ListApplications())
	d := views.CVData{Config: s.store.Config(), Error: msg}
	for _, f := range files {
		item := views.CVItem{Name: f.Name, Size: f.Size, ModTime: f.ModTime}
		for _, row := range rows {
			if row.App.Job.CVVersion == f.Name {
				item.UsedIn = append(item.UsedIn, row)
			}
		}
		d.Files = append(d.Files, item)
	}
	s.render(w, r, status, views.CVPage(s.page("CV library", "cv"), d))
}

func (s *Server) uploadCV(w http.ResponseWriter, r *http.Request) {
	file, hdr, msg := readUpload(w, r)
	if msg != "" {
		s.renderCVs(w, r, http.StatusBadRequest, msg)
		return
	}
	defer file.Close()
	defer r.MultipartForm.RemoveAll()
	if _, err := s.store.SaveCV(hdr.Filename, file); err != nil {
		s.serverError(w, r, err)
		return
	}
	redirect(w, r, "/cv")
}

func (s *Server) downloadCV(w http.ResponseWriter, r *http.Request) {
	f, err := s.store.OpenCV(r.PathValue("name"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.notFound(w, r)
			return
		}
		s.serverError(w, r, err)
		return
	}
	defer f.Close()
	serveFile(w, r, f, r.PathValue("name"))
}
