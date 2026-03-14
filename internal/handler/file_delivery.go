package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"uzeltok/internal/model"
	"uzeltok/internal/service"
	"uzeltok/internal/store"
)

type fileDelivery struct {
	links *service.LinkService
}

func newFileDelivery(links *service.LinkService) *fileDelivery {
	return &fileDelivery{links: links}
}

func (d *fileDelivery) Serve(w http.ResponseWriter, r *http.Request, l *model.Link, filename string, notFound func(http.ResponseWriter, *http.Request)) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")

	rc, err := d.links.OpenFile(l, filename)
	if err != nil {
		if err == store.ErrNotFound || err == store.ErrInvalidPath {
			notFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rc.Close()

	var modtime time.Time
	var size int64

	fallbackModtime := time.Time{}
	for _, f := range l.Files {
		if f.Name == filename {
			fallbackModtime = f.Timestamp
			size = f.Size
			break
		}
	}

	if rs, ok := rc.(io.ReadSeeker); ok {
		if f, ok := rc.(*os.File); ok {
			if st, err := f.Stat(); err == nil {
				modtime = st.ModTime()
				size = st.Size()
			}
		}

		if modtime.IsZero() {
			modtime = fallbackModtime
		}

		w.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, modtime.UnixNano(), size))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.NewReplacer(`\\`, `\\\\`, `"`, `\\"`).Replace(filename)))

		http.ServeContent(w, r, filename, modtime, rs)
		return
	}

	data, err := io.ReadAll(rc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if size == 0 {
		size = int64(len(data))
	}
	modtime = fallbackModtime

	w.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, modtime.UnixNano(), size))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.NewReplacer(`\\`, `\\\\`, `"`, `\\"`).Replace(filename)))

	http.ServeContent(w, r, filename, modtime, bytes.NewReader(data))
}
