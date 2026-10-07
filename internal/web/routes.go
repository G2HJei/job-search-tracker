package web

import "net/http"

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.dashboard)

	mux.HandleFunc("GET /applications", s.listApplications)
	mux.HandleFunc("GET /applications/new", s.newApplication)
	mux.HandleFunc("POST /applications", s.createApplication)
	mux.HandleFunc("GET /applications/{id}", s.showApplication)
	mux.HandleFunc("GET /applications/{id}/sections/{section}", s.sectionView)
	mux.HandleFunc("GET /applications/{id}/sections/{section}/edit", s.sectionEdit)
	mux.HandleFunc("POST /applications/{id}/sections/{section}", s.sectionSave)
	mux.HandleFunc("POST /applications/{id}/status", s.setStatus)
	mux.HandleFunc("POST /applications/{id}/priority", s.setPriority)
	mux.HandleFunc("POST /applications/{id}/touch", s.touch)
	mux.HandleFunc("POST /applications/{id}/company", s.relinkCompany)
	mux.HandleFunc("POST /applications/{id}/delete", s.deleteApplication)
	mux.HandleFunc("POST /applications/{id}/files", s.uploadFile)
	mux.HandleFunc("POST /applications/{id}/posting-text", s.savePostingText)
	mux.HandleFunc("GET /applications/{id}/files/{name}", s.downloadFile)
	mux.HandleFunc("POST /applications/{id}/files/{name}/delete", s.deleteFile)
	mux.HandleFunc("GET /partials/row/{kind}", s.blankRow)

	mux.HandleFunc("GET /board", s.board)
	mux.HandleFunc("GET /calendar", s.calendar)
	mux.HandleFunc("GET /calendar.ics", s.calendarICS)

	mux.HandleFunc("GET /companies", s.listCompanies)
	mux.HandleFunc("GET /companies/new", s.newCompany)
	mux.HandleFunc("POST /companies", s.createCompany)
	mux.HandleFunc("GET /companies/{id}", s.showCompany)
	mux.HandleFunc("GET /companies/{id}/edit", s.editCompany)
	mux.HandleFunc("POST /companies/{id}", s.updateCompany)

	mux.HandleFunc("GET /cv", s.listCVs)
	mux.HandleFunc("POST /cv", s.uploadCV)
	mux.HandleFunc("GET /cv/{name}", s.downloadCV)

	mux.HandleFunc("GET /export.csv", s.exportCSV)
	mux.HandleFunc("GET /backup.zip", s.backupZip)
	mux.HandleFunc("GET /settings", s.settings)
	mux.HandleFunc("POST /settings/reload", s.reload)

	mux.Handle("GET /static/", s.staticHandler())
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/favicon.svg", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/", s.notFound)
	return mux
}
