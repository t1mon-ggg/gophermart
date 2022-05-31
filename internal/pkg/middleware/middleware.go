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

var sublog = log.With().Str("component", "middleware").Logger()

//TimeTracer - middleware for time tracking processing incoming requests
func TimeTracer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tStart := time.Now()
		sublog.Debug().Msgf("Request processing start time is %v", tStart.Format(time.RFC3339))
		next.ServeHTTP(w, r)
		duration := time.Since(tStart)
		tEnd := tStart.Add(duration)
		sublog.Debug().Msgf("Request processing end time is %v", tEnd.Format(time.RFC3339))
		sublog.Info().Msgf("Duration for a request %s", duration)
	})
}

//DecompressRequest - middleware for decompressing incoming user requests on the fly
func DecompressRequest(next http.Handler) http.Handler {
	sublog.Debug().Msg("Request decompression")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (strings.Contains(r.Header.Get("Content-Encoding"), "gzip")) || (strings.Contains(r.Header.Get("Content-Encoding"), "br")) || (strings.Contains(r.Header.Get("Content-Encoding"), "deflate")) {
			sublog.Debug().Msg("Compression header found")
			defer r.Body.Close()
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				sublog.Error().Err(err).Msg("Could not create reader of compressed data")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			sublog.Debug().Msg("zip reader created")
			defer gz.Close()
			body, err := io.ReadAll(gz)
			if err != nil {
				sublog.Error().Err(err).Msg("Could not read compressed data")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			sublog.Debug().Msg("request body read")
			r.ContentLength = int64(len(body))
			log.Debug().Msgf("New ContentLenght is %v", int64(len(body)))
			r.Body = io.NopCloser(bytes.NewBuffer(body))
			log.Debug().Msgf("New request body is %v", string(body))
			sublog.Debug().Msg("decompressiong complete")
		}
		next.ServeHTTP(w, r)
	})
}
