package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// Gzip wraps server handlers with gzip support:
//   - reads gzipped request bodies when Content-Encoding contains gzip,
//   - writes gzipped responses for clients that accept gzip
//     and for supported content types.
func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if headerContainsToken(r.Header.Get("Content-Encoding"), "gzip") {
			gzipReader, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			r.Body = &gzipReadCloser{
				Reader: gzipReader,
				body:   r.Body,
			}
		}

		gzipWriter := newGzipResponseWriter(w, headerContainsToken(r.Header.Get("Accept-Encoding"), "gzip"))
		defer gzipWriter.Close()

		next.ServeHTTP(gzipWriter, r)
	})
}

type gzipReadCloser struct {
	Reader io.ReadCloser
	body   io.Closer
}

func (g *gzipReadCloser) Read(p []byte) (int, error) {
	return g.Reader.Read(p)
}

func (g *gzipReadCloser) Close() error {
	readerErr := g.Reader.Close()
	bodyErr := g.body.Close()
	if readerErr != nil {
		return readerErr
	}
	return bodyErr
}

type gzipResponseWriter struct {
	http.ResponseWriter
	acceptsGzip bool
	wroteHeader bool
	compressing bool
	writer      *gzip.Writer
}

func newGzipResponseWriter(w http.ResponseWriter, acceptsGzip bool) *gzipResponseWriter {
	return &gzipResponseWriter{
		ResponseWriter: w,
		acceptsGzip:    acceptsGzip,
	}
}

func (g *gzipResponseWriter) WriteHeader(statusCode int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true

	contentType := g.Header().Get("Content-Type")
	if g.acceptsGzip && canCompressContentType(contentType) {
		g.compressing = true
		g.Header().Set("Content-Encoding", "gzip")
		g.Header().Add("Vary", "Accept-Encoding")
		g.Header().Del("Content-Length")
		g.writer = gzip.NewWriter(g.ResponseWriter)
	}

	g.ResponseWriter.WriteHeader(statusCode)
}

func (g *gzipResponseWriter) Write(body []byte) (int, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.compressing {
		return g.writer.Write(body)
	}
	return g.ResponseWriter.Write(body)
}

func (g *gzipResponseWriter) Close() error {
	if g.writer != nil {
		return g.writer.Close()
	}
	return nil
}

func canCompressContentType(contentType string) bool {
	return strings.HasPrefix(contentType, "application/json") ||
		strings.HasPrefix(contentType, "text/html")
}

func headerContainsToken(headerValue, token string) bool {
	for _, part := range strings.Split(strings.ToLower(headerValue), ",") {
		if strings.TrimSpace(part) == strings.ToLower(token) {
			return true
		}
	}
	return false
}
