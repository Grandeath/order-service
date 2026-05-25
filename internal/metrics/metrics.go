package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	Paths = newPaths(factory)

	// queryLabel     = "label"
	// defaultBuckets = []float64{.002, .005, .01, .025, .05, 0.085, .1, .125, .150, .170, .180, .190, .200, .250}

	reg     = prometheus.WrapRegistererWithPrefix("app_", prometheus.DefaultRegisterer)
	factory = promauto.With(reg)
)

func GetRegisterer() prometheus.Registerer {
	return reg
}

func newPaths(factory promauto.Factory) *paths {
	return &paths{
		path: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "paths",
			},
			[]string{"pathname"}),
		methodNotAllowed: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "paths_method_not_allowed",
			},
			[]string{"pathname"}),
		notFound: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "paths_not_found",
			}),
	}
}

type paths struct {
	path             *prometheus.CounterVec
	methodNotAllowed *prometheus.CounterVec
	notFound         prometheus.Counter
}

func (p *paths) Log(path string) {
	p.path.WithLabelValues(path).Inc()
}

func (p *paths) MethodNotAllowed(path string) {
	p.methodNotAllowed.WithLabelValues(path).Inc()
}

func (p *paths) NotFound() {
	p.notFound.Inc()
}
