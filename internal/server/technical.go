package server

import (
	"html/template"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const defaultTimeout = 15 * time.Second

type TechConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func TechnicalEndpoints() []Endpoint {
	endpoints := []Endpoint{
		{
			Path:    "/ping",
			Handler: ping,
		},
		{
			Path:    "/debug/pprof/",
			Handler: pprof.Index,
		},
		{
			Path:    "/debug/pprof/cmdline",
			Handler: pprof.Cmdline,
		},
		{
			Path:    "/debug/pprof/profile",
			Handler: pprof.Profile,
		},
		{
			Path:    "/debug/pprof/trace",
			Handler: pprof.Trace,
		},
		{
			Path:    "/metrics",
			Handler: promhttp.Handler().ServeHTTP,
		},
	}

	return endpoints
}

func ping(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("pong"))
}

func index(endpoints []Endpoint) func(w http.ResponseWriter, r *http.Request) {
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
