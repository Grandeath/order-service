package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kprom"
)

const compressionLevel = 3

type EventNotifierConfig struct {
	Enabled          bool
	URL              string
	Topic            string
	Registerer       prometheus.Registerer
	MetricsNameSpace string
	Compression      bool
}

type EventNotifier struct {
	enabled  bool
	producer *kgo.Client
}

func NewEventNotifier(cfg EventNotifierConfig) (*EventNotifier, error) {
	if !cfg.Enabled {
		return &EventNotifier{
			enabled: false,
		}, nil
	}

	if cfg.MetricsNameSpace == "" {
		return nil, errors.New("metrics namespace is required")
	}

	opts := kafkaOptionsNotifier(cfg.Topic, cfg.URL)
	if cfg.Registerer != nil {
		opts = append(opts, kgo.WithHooks(kafkaHooksNotifier(cfg.Registerer, cfg.MetricsNameSpace)))
	}

	if cfg.Compression {
		opts = append(opts, kgo.ProducerBatchCompression(kgo.ZstdCompression().WithLevel(compressionLevel)))
	}

	kafkaProducer, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create kafka producer: %w", err)
	}

	return &EventNotifier{
		enabled:  true,
		producer: kafkaProducer,
	}, nil
}

func (n *EventNotifier) Notify(log any) error {
	if !n.enabled {
		return nil
	}
	b, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("unable to marshal log: %w", err)
	}
	n.producer.Produce(context.Background(), kgo.SliceRecord(b), nil)

	return nil
}

func (n *EventNotifier) Close() {
	if n != nil && n.producer != nil {
		n.producer.Close()
	}
}

func kafkaHooksNotifier(registerer prometheus.Registerer, namespaceEventNotifier string) *kprom.Metrics {
	kpromOpts := []kprom.Opt{
		kprom.Registerer(registerer),
		kprom.FetchAndProduceDetail(kprom.ByNode, kprom.Batches, kprom.Records),
		kprom.Histograms(
			kprom.WriteWait,
			kprom.RequestDurationE2E,
		),
	}
	return kprom.NewMetrics(namespaceEventNotifier, kpromOpts...)
}

func kafkaOptionsNotifier(topic, urls string) []kgo.Opt {
	return []kgo.Opt{
		kgo.SeedBrokers(strings.Split(urls, ",")...),
		kgo.DefaultProduceTopic(topic),
		kgo.ProducerLinger(time.Duration(50) * time.Millisecond),
		kgo.MaxBufferedRecords(20_000),
		kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)),
	}
}
