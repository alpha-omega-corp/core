package app

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/uptrace/bunrouter"
	"github.com/uptrace/bunrouter/extra/bunrouterotel"
	"github.com/uptrace/bunrouter/extra/reqlog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func CreateHTTP(config *Config, init func(router *bunrouter.Router)) {
	router := bunrouter.New(
		bunrouter.WithMiddleware(reqlog.NewMiddleware(
			reqlog.WithEnabled(true),
			reqlog.WithVerbose(true),
		)),

		bunrouter.Use(bunrouterotel.NewMiddleware(
			bunrouterotel.WithClientIP(),
		)))

	init(router)

	handler := otelhttp.NewHandler(router, "")
	httpSrv := &http.Server{
		Addr:         config.Url,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
		Handler:      handler,
	}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
			log.Fatalf("ListenAndServe failed: %s", err)
		}
	}()

	fmt.Printf("listening on http://%s\n", httpSrv.Addr)
}
