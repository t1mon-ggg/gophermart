package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

//TimeTracer - middleware для остлеживания времени исполнения запроса
func TimeTracer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tStart := time.Now()
		next.ServeHTTP(w, r)
		tEnd := time.Since(tStart)
		log.Info().Msgf("Duration for a request %s", tEnd)
	})
}

//DecompressRequest - middleware для декомпрессии входящих запросов
func DecompressRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (strings.Contains(r.Header.Get("Content-Encoding"), "gzip")) || (strings.Contains(r.Header.Get("Content-Encoding"), "br")) || (strings.Contains(r.Header.Get("Content-Encoding"), "deflate")) {
			defer r.Body.Close()
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				log.Error().Err(err).Msg("Could not create reader of compressed data")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			defer gz.Close()
			body, err := io.ReadAll(gz)
			if err != nil {
				log.Error().Err(err).Msg("Could not read compressed data")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			r.ContentLength = int64(len(body))
			r.Body = io.NopCloser(bytes.NewBuffer(body))
		}
		next.ServeHTTP(w, r)
	})
}
