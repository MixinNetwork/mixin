package main

import (
	"fmt"
	"net/http"

	"github.com/bugsnag/bugsnag-go"
	"github.com/dimfeld/httptreemux"
	"github.com/facebookgo/grace/gracehttp"
	"github.com/gorilla/handlers"
	"github.com/unrolled/render"
	"mixin.one/durable"
	"mixin.one/middlewares"
	"mixin.one/routes"
)

func StartHTTP() error {
	logger, err := durable.NewLoggerClient("block-api", true)
	if err != nil {
		return err
	}
	defer logger.Close()

	router := httptreemux.New()
	routes.RegisterHanders(router)
	RegisterRoutes(router)
	handler := AuthenticateMiddleware(router)
	handler = middlewares.Constraint(handler)
	handler = middlewares.Context(handler, nil, nil, render.New())
	handler = middlewares.Stats(handler, "http", true, "0.1")
	handler = middlewares.Log(handler, logger, "http")
	handler = handlers.ProxyHeaders(handler)
	handler = bugsnag.Handler(handler)

	return gracehttp.Serve(&http.Server{Addr: fmt.Sprintf(":%d", 7001), Handler: handler})
}
