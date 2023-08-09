package main

import (
	"database/sql"
	"github.com/Kreg101/alice-skill/internal/logger"
	"github.com/Kreg101/alice-skill/internal/store/pg"
	"go.uber.org/zap"
	"net/http"
	"strings"

	_ "github.com/jackc/pgx/v5"
)

func main() {

	parseFlags()

	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	if err := logger.Initialize(flagLogLevel); err != nil {
		return err
	}

	// создаём соединение к СУБД PostgreSQL с помощью аргумента командной строки
	conn, err := sql.Open("pgx", flagDatabaseURI)
	if err != nil {
		return err
	}

	// создаём экземпляр приложения, передавая реализацию хранилища pg в качестве внешней зависимости
	appInstance := newApp(pg.NewStore(conn))

	logger.Log.Info("Running server", zap.String("address", flagRunAddr))
	// обернём хендлер webhook в middleware с логгированием и поддержкой gzip
	return http.ListenAndServe(flagRunAddr, logger.RequestLogger(gzipMiddleware(appInstance.webhook)))
}

func gzipMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ow := w

		// проверяем, что клиент умеет получать от сервера сжатые данные в формате gzip
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")
		if supportsGzip {
			cw := newCompressWriter(w)
			ow = cw
			defer cw.Close()
		}

		// проверяем, что клиент отправил серверу сжатые данные в формате gzip
		contentEncoding := r.Header.Get("Content-Encoding")
		sendsGzip := strings.Contains(contentEncoding, "gzip")
		if sendsGzip {
			cr, err := newCompressReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			r.Body = cr
			defer cr.Close()
		}

		h.ServeHTTP(ow, r)
	}
}
