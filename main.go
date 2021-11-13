package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/number"
	"go.opentelemetry.io/otel/metric/sdkapi"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/credentials"

	"net/http"

	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	"github.com/sensu/sensu-go/types"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkexport "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
)

// Config represents the handler plugin config.
type Config struct {
	sensu.PluginConfig
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "otel-sensu-handler-plugin",
			Short:    "Generate OpenTelemetry metrics from Sensu",
			Keyspace: "sensu.io/plugins/otel-sensu-handler-plugin/config",
		},
	}
	port    = ":55788"
	options []*sensu.PluginConfigOption
)

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

type otelPlugin struct {
	*resource.Resource
	*otlpmetric.Exporter
}

type exportEvent struct {
	*types.Event
}

type exportValue struct {
	value     float64
	timestamp time.Time
}

type exportLibraryEvent struct {
	sync.RWMutex
	*types.Event
}

func main() {
	ctx := context.Background()
	otelExporter, err := otlpmetric.New(
		ctx,
		otlpmetricgrpc.NewClient(
			otlpmetricgrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")),
			otlpmetricgrpc.WithEndpoint(getenv("OTEL_EXPORTER_OTLP_METRIC_ENDPOINT", "ingest.lightstep.com:443")),
			otlpmetricgrpc.WithHeaders(map[string]string{
				"lightstep-access-token": os.Getenv("LS_ACCESS_TOKEN"),
			}),
		),
	)

	if err != nil {
		log.Fatalf("failed to initialize otelgrpc pipeline: %v", err)
	}

	ot := &otelPlugin{
		Resource: resource.Empty(),
		Exporter: otelExporter,
	}

	if os.Getenv("ENABLE_SENSU_HANDLER") == "1" {
		log.Printf("starting sensu handler...")
		handler := sensu.NewGoHandler(&plugin.PluginConfig, options, checkArgs, ot.executeHandler)
		handler.Execute()
	} else {
		log.Printf("starting http server on port %v...", port)
		http.HandleFunc("/", ot.postEvent)
		err = http.ListenAndServe(port, nil)
		if err != nil {
			log.Fatalf("could not listed on port: %v", err.Error())
		}
	}
}

func checkArgs(_ *types.Event) error {
	if len(os.Getenv("LS_ACCESS_TOKEN")) == 0 {
		return fmt.Errorf("LS_ACCESS_TOKEN is not set")
	}
	return nil
}

func (ot *otelPlugin) eventToOtel(event *types.Event) error {
	return ot.Exporter.Export(
		context.Background(),
		ot.Resource,
		&exportEvent{Event: event},
	)
}

func (ex *exportEvent) ForEach(readerFunc func(instrumentation.Library, sdkexport.Reader) error) error {
	return readerFunc(instrumentation.Library{
		Name: "sensu-otel",
	}, &exportLibraryEvent{Event: ex.Event})
}

func (ex *exportValue) Kind() aggregation.Kind {
	return aggregation.LastValueKind
}

func (ex *exportValue) LastValue() (number.Number, time.Time, error) {
	return number.NewFloat64Number(ex.value), ex.timestamp, nil
}

func (ex *exportLibraryEvent) ForEach(_ aggregation.TemporalitySelector, recordFunc func(sdkexport.Record) error) error {
	for _, m := range ex.Event.Metrics.Points {
		var attrs []attribute.KeyValue
		for _, t := range m.Tags {
			attrs = append(attrs, attribute.String(t.Name, t.Value))
		}
		descriptor := sdkapi.NewDescriptor(m.Name, sdkapi.GaugeObserverInstrumentKind, number.Float64Kind, "", "")

		attrSet := attribute.NewSet(attrs...)

		gauge := exportValue{
			value:     m.Value,
			timestamp: time.Unix(0, m.Timestamp), // Timestamp is in nanoseconds
		}

		log.Printf("recording metric: %v=%v\n", m.Name, m.Value)

		if err := recordFunc(
			sdkexport.NewRecord(
				&descriptor,
				&attrSet,
				&gauge,
				gauge.timestamp.Add(-time.Microsecond),
				gauge.timestamp,
			)); err != nil {
			return err
		}
	}
	return nil
}

// curl --data '@test-event.json' http://localhost:55788
func (ot *otelPlugin) postEvent(w http.ResponseWriter, req *http.Request) {
	var e types.Event
	err := json.NewDecoder(req.Body).Decode(&e)
	if err != nil {
		http.Error(w, fmt.Sprintf("event parse error: %v", err.Error()), http.StatusBadRequest)
		return
	}
	err = ot.eventToOtel(&e)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not convert event to otel: %v", err.Error()), http.StatusBadRequest)
	}
	fmt.Fprintf(w, "ok: %v\n", e.Metrics)
}

// based on: https://github.com/portertech/sensu-prometheus-pushgateway-handler/blob/main/main.go
func (ot *otelPlugin) executeHandler(event *types.Event) error {
	err := ot.eventToOtel(event)
	if err != nil {
		return err
	}
	return nil
}
