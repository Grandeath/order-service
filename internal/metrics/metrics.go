package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Defining metrics as global const allows to avoid panics in unit tests
	Counter = newCounter(factory)

	queryLabel     = "label"
	defaultBuckets = []float64{.002, .005, .01, .025, .05, 0.085, .1, .125, .150, .170, .180, .190, .200, .250}

	reg     = prometheus.WrapRegistererWithPrefix("app_", prometheus.DefaultRegisterer)
	factory = promauto.With(reg)
)

func GetRegisterer() prometheus.Registerer {
	return reg
}

type counter struct {
	counter     *prometheus.CounterVec
	durationVec *prometheus.HistogramVec
	duration    prometheus.Histogram
}

func newCounter(factory promauto.Factory) *counter {
	return &counter{
		counter: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "counter",
			Help: "Counter description",
		}, []string{queryLabel}),
		durationVec: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "duration_vec",
			Help:    "Duration vec description",
			Buckets: defaultBuckets,
		}, []string{queryLabel}),
		duration: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "duration",
			Help:    "Duration description",
			Buckets: defaultBuckets,
		}),
	}
}

func (c *counter) Request() {
	c.counter.WithLabelValues("request").Inc()
}

// ObserveDuration return function which records the duration passed since this method was called.
func (c *counter) ObserveDuration() (doneFunc func()) {
	prometheusTimer := prometheus.NewTimer(c.duration)
	return func() {
		prometheusTimer.ObserveDuration()
	}
}

func (c *counter) ObserveDurationVec(label string) (doneFunc func()) {
	t := time.Now()
	return func() {
		d := time.Since(t)
		c.durationVec.WithLabelValues(label).Observe(d.Seconds())
	}
}
