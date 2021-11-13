module github.com/smithclay/otel-sensu-handler-plugin

go 1.14

replace go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc => ../go/exporters/otlp/otlpmetric/otlpmetricgrpc

require (
	github.com/sensu-community/sensu-plugin-sdk v0.11.0
	github.com/sensu/sensu-go/api/core/v2 v2.3.0
	github.com/sensu/sensu-go/types v0.3.0
	go.opentelemetry.io/otel v1.2.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric v0.25.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v0.24.0
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v0.24.0
	go.opentelemetry.io/otel/metric v0.25.0
	go.opentelemetry.io/otel/sdk v1.2.0
	go.opentelemetry.io/otel/sdk/export/metric v0.25.0
	go.opentelemetry.io/otel/sdk/metric v0.25.0
	google.golang.org/grpc v1.42.0
)
