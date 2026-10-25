package gzip

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"
)

var writerPool = sync.Pool{
	New: func() any { return nil },
}

const (
	headerContentType     = "Content-Type"
	headerContentEncoding = "Content-Encoding"
	headerAcceptEncoding  = "Accept-Encoding"
	headerVary            = "Vary"
)

var skipContentTypes = map[string]bool{
	"application/gzip": true,
	"application/zip":  true,
	"image/png":        true,
	"image/jpeg":       true,
	"video/mp4":        true,
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "bad gzip body", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			r.Body = gz
		}

		if !acceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Add(headerVary, headerAcceptEncoding)
		gz := getWriter(w)
		gw := &gzipResponseWriter{ResponseWriter: w, gz: gz}
		next.ServeHTTP(gw, r)

		if gw.used {
			putWriter(gz)
		} else {
			writerPool.Put(gz)
		}
	})
}

func acceptsGzip(r *http.Request) bool {
	ae := r.Header.Get(headerAcceptEncoding)
	if ae == "" {
		return false
	}
	for _, part := range strings.Split(ae, ",") {
		enc := strings.TrimSpace(part)
		if i := strings.IndexByte(enc, ';'); i >= 0 {
			enc = strings.TrimSpace(enc[:i])
		}
		if strings.EqualFold(enc, "gzip") || enc == "*" {
			return true
		}
	}
	return false
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz        *gzip.Writer
	wroteHead bool
	used      bool
	compress  bool
	status    int
}

func (g *gzipResponseWriter) Header() http.Header {
	return g.ResponseWriter.Header()
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	if g.wroteHead {
		return
	}
	g.wroteHead = true
	g.status = code

	g.compress = !skipCompress(g.Header().Get(headerContentType)) &&
		code != http.StatusNoContent &&
		code != http.StatusNotModified
	g.ResponseWriter.Header().Del("Content-Length")
	if g.compress {
		g.ResponseWriter.Header().Set(headerContentEncoding, "gzip")
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHead {
		g.WriteHeader(http.StatusOK)
	}
	if !g.compress {
		return g.ResponseWriter.Write(b)
	}
	g.used = true
	return g.gz.Write(b)
}

func skipCompress(contentType string) bool {
	ct := strings.TrimSpace(contentType)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return skipContentTypes[strings.ToLower(ct)]
}

func getWriter(w http.ResponseWriter) *gzip.Writer {
	if v := writerPool.Get(); v != nil {
		gz := v.(*gzip.Writer)
		gz.Reset(w)
		return gz
	}
	return gzip.NewWriter(w)
}

func putWriter(gz *gzip.Writer) {
	_ = gz.Close()
	writerPool.Put(gz)
}
