package handler

import (
	"errors"
	"net/http"
)

func (h *Handler) saveMultipartUpload(w http.ResponseWriter, r *http.Request, linkID string) (int, string) {
	if r.ContentLength > h.cfg.MaxUploadBytes {
		return http.StatusRequestEntityTooLarge, "request body too large"
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxUploadBytes)
	if err := r.ParseMultipartForm(h.cfg.MaxUploadBytes); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return http.StatusRequestEntityTooLarge, "request body too large"
		}
		return http.StatusBadRequest, "failed to parse form: " + err.Error()
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		return http.StatusBadRequest, "no files provided"
	}

	if err := h.uploads.SaveMultipartFiles(linkID, files); err != nil {
		return http.StatusInternalServerError, "failed to save file: " + err.Error()
	}

	return 0, ""
}
