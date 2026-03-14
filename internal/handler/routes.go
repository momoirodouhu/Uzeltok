package handler

import (
	"fmt"
	"net/http"
)

func (h *Handler) registerAdminRoutes(mux *http.ServeMux) {
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return h.adminAuth(next)
	}

	wrap := func(next http.HandlerFunc) http.HandlerFunc {
		return methodOverride(next).ServeHTTP
	}

	mux.HandleFunc("GET /admin", auth(h.handleAdmin))
	mux.HandleFunc("POST /admin/tus/gc", auth(h.csrfProtect(h.handleAdminRunTusGC)))
	mux.HandleFunc("POST /admin/links", auth(h.csrfProtect(wrap(h.handleAdminCreateLink))))
	mux.HandleFunc("GET /admin/links/{id}", auth(h.handleAdminDetail))
	mux.HandleFunc("POST /admin/links/{id}", auth(h.csrfProtect(wrap(h.handleAdminDeleteLink))))
	mux.HandleFunc("POST /admin/links/{id}/files", auth(h.csrfProtect(h.handleAdminUpload)))
	mux.HandleFunc("POST /admin/links/{id}/files/{filename}", auth(h.csrfProtect(wrap(h.handleAdminDeleteFile))))
	mux.HandleFunc("GET /admin/links/{id}/files/{filename}", auth(h.handleAdminDownloadFile))
	mux.HandleFunc("OPTIONS /admin/links/{id}/tus", auth(h.handleAdminTus))
	mux.HandleFunc("HEAD /admin/links/{id}/tus/{uploadID...}", auth(h.handleAdminTus))
	mux.HandleFunc("POST /admin/links/{id}/tus", auth(h.csrfProtect(h.handleAdminTus)))
	mux.HandleFunc("PATCH /admin/links/{id}/tus/{uploadID...}", auth(h.csrfProtect(h.handleAdminTus)))
	mux.HandleFunc("DELETE /admin/links/{id}/tus/{uploadID...}", auth(h.csrfProtect(h.handleAdminTus)))
}

func (h *Handler) registerPublicRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.handleIndex)
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		fmt.Fprint(w, "User-agent: *\nDisallow: /\n")
	})
	mux.HandleFunc("GET /{id}", h.handleLinkDetail)
	mux.HandleFunc("POST /{id}/files", h.handlePublicUpload)
	mux.HandleFunc("GET /{id}/files/{filename}", h.handlePublicDownload)
	mux.HandleFunc("OPTIONS /{id}/tus", h.handlePublicTus)
	mux.HandleFunc("POST /{id}/tus", h.handlePublicTus)
	mux.HandleFunc("HEAD /{id}/tus/{uploadID...}", h.handlePublicTus)
	mux.HandleFunc("PATCH /{id}/tus/{uploadID...}", h.handlePublicTus)
	mux.HandleFunc("DELETE /{id}/tus/{uploadID...}", h.handlePublicTus)
}


