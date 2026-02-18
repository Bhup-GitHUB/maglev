package restapi

import (
	"fmt"
	"net/http"
)

// CacheControlMiddleware adds Cache-Control headers based on the tier
func CacheControlMiddleware(durationSeconds int, next http.Handler) http.Handler {
	var headerValue string
	if durationSeconds > 0 {
		headerValue = fmt.Sprintf("public, max-age=%d", durationSeconds)
	} else {
		headerValue = "no-cache, no-store, must-revalidate"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := &cacheControlWriter{
			ResponseWriter: w,
			headerValue:    headerValue,
		}
		next.ServeHTTP(wrapped, r)
	})
}

type cacheControlWriter struct {
	http.ResponseWriter
	headerValue   string
	headerWritten bool
}

func (w *cacheControlWriter) WriteHeader(code int) {
	if !w.headerWritten {
		w.headerWritten = true
		if code >= 200 && code < 300 {
			w.ResponseWriter.Header().Set("Cache-Control", w.headerValue)
		} else {
			w.ResponseWriter.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *cacheControlWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
