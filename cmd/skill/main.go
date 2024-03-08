package main

import (
	"bitbucket.org/sotavant/yandex-alice-skill/internal/logger"
	"bitbucket.org/sotavant/yandex-alice-skill/internal/models"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

func main() {
	parseFlags()
	if err := run(); err != nil {
		panic(err)
	}
}

func gzipMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ow := w

		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportGzip := strings.Contains(acceptEncoding, "gzip")

		if supportGzip {
			cw := newCompressWriter(w)
			ow = cw
			defer func(cw *compressWriter) {
				err := cw.Close()
				if err != nil {
					logger.Log.Debug("compressWriterError", zap.Error(err))
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}(cw)
		}

		contentEncoding := r.Header.Get("Content-Encoding")

		sendsGzip := strings.Contains(contentEncoding, "gzip")
		if sendsGzip {
			cr, err := newCompressReader(r.Body)
			if err != nil {
				logger.Log.Debug("newCompressReaderError", zap.Error(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			r.Body = cr
			defer func(cr *compressReader) {
				err := cr.Close()
				if err != nil {
					logger.Log.Debug("closeCompressReaderError", zap.Error(err))
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}(cr)
		}

		h.ServeHTTP(ow, r)
	}
}
func run() error {
	if err := logger.Initialize(flagLogLevel); err != nil {
		return err
	}

	logger.Log.Info("Running server", zap.String("address", flagRunAddr))

	return http.ListenAndServe(flagRunAddr, logger.RequestLogger(gzipMiddleware(webhook)))
}

func webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.Log.Debug("got request with bad method", zap.String("method", r.Method))

		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	logger.Log.Debug("decodint request")
	var req models.Request
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		logger.Log.Debug("cannto decode request JSON body", zap.Error(err))

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if req.Request.Type != models.TypeSimpleUtterance {
		logger.Log.Debug("usupported request type", zap.String("type", req.Request.Type))
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	text := "Для вас нет новых сообщений."

	if req.Session.New {
		tz, err := time.LoadLocation(req.Timezone)
		if err != nil {
			logger.Log.Debug("cannot parse timezone")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		now := time.Now().In(tz)
		hour, minute, _ := now.Clock()
		text = fmt.Sprintf("Точное время %d часов, %d минут. %s", hour, minute, text)
	}

	resp := models.Response{
		Response: models.ResponsePayload{Text: text},
		Version:  "1.0",
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(resp); err != nil {
		logger.Log.Debug("error encoding response", zap.Error(err))
		return
	}

	logger.Log.Debug("sending http 200 response")
}
