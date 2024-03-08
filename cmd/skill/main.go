package main

import (
	"bitbucket.org/sotavant/yandex-alice-skill/internal/logger"
	"go.uber.org/zap"
	"net/http"
	"strings"
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

	appInstance := newApp(nil)

	logger.Log.Info("Running server", zap.String("address", flagRunAddr))

	return http.ListenAndServe(flagRunAddr, logger.RequestLogger(gzipMiddleware(appInstance.webhook)))
}
