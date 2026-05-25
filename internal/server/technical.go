package server

import (
	"html/template"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func TechnicalEndpoints() []*Endpoint {
	endpoints := []*Endpoint{
		{
			Method:  http.MethodGet,
			Path:    "/ping",
			Handler: ping,
		},
		{
			Method:  http.MethodGet,
			Path:    "/debug/pprof/",
			Handler: pprof.Index,
		},
		{
			Method:  http.MethodGet,
			Path:    "/debug/pprof/cmdline",
			Handler: pprof.Cmdline,
		},
		{
			Method:  http.MethodGet,
			Path:    "/debug/pprof/profile",
			Handler: pprof.Profile,
		},
		{
			Method:  http.MethodGet,
			Path:    "/debug/pprof/trace",
			Handler: pprof.Trace,
		},
		{
			Method:  http.MethodGet,
			Path:    "/metrics",
			Handler: promhttp.Handler().ServeHTTP,
		},
		{
			Method:  http.MethodGet,
			Path:    "/version",
			Handler: appVersion,
		},
	}

	return endpoints
}

func TechMiddlewares() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{middleware.NoCache}
}

func index(endpoints []*Endpoint) http.HandlerFunc {
	var tmpl = `<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta http-equiv="X-UA-Compatible" content="IE=edge">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Index</title>
	</head>
	<body>
		<ul>
			{{range .Endpoints}}
				<li><a href="{{.}}">{{.}}</a></li>
			{{end}}
		</ul>    
	</body>
	</html>`

	t, err := template.New("").Parse(tmpl)
	if err != nil {
		slog.Error("could not parse the index template: ", "error", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var paths []string
		for _, route := range endpoints {
			paths = append(paths, route.Path)
		}
		if t == nil {
			return
		}
		if err := t.Execute(w, map[string]any{"Endpoints": paths}); err != nil {
			slog.Error("could not execute the index template: ", "error", err)
		}
	}
}
