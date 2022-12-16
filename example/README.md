# Example

This package shows how the `otlpr` logger operates.

## Running

From this directory run the OpenTelemetry collector so it can receive the log messages the example application generates.

```terminal
docker run --name otel-collector -d -p "4317:4317" -v $(pwd)/config.yaml:/etc/otelcol/config.yaml otel/opentelemetry-collector
```

Next, generate logs by running the application.

```terminal
go run . -target="127.0.0.1:4317"
```

The collector logs should contain the generated log messages.
Check using the docker utility.

``` terminal
docker logs otel-collector
```

For example:

> 2022-12-16T19:47:53.473Z	info	LogsExporter	{"kind": "exporter", "data_type": "logs", "name": "logging", "#logs": 1}
> 2022-12-16T19:47:53.473Z	info	ResourceLog #0
> Resource SchemaURL:
> ScopeLogs #0
> ScopeLogs SchemaURL:
> InstrumentationScope
> LogRecord #0
> ObservedTimestamp: 1970-01-01 00:00:00 +0000 UTC
> Timestamp: 2022-12-16 19:47:53.47290946 +0000 UTC
> SeverityText: 0
> SeverityNumber: Warn4(16)
> Body: Str(information message)
> Attributes:
>      -> function: Str(main)
> Trace ID:
> Span ID:
> Flags: 0
> 	{"kind": "exporter", "data_type": "logs", "name": "logging"}
> 2022-12-16T19:47:53.474Z	info	LogsExporter	{"kind": "exporter", "data_type": "logs", "name": "logging", "#logs": 1}
> 2022-12-16T19:47:53.474Z	info	ResourceLog #0
> Resource SchemaURL:
> ScopeLogs #0
> ScopeLogs SchemaURL:
> InstrumentationScope
> LogRecord #0
> ObservedTimestamp: 1970-01-01 00:00:00 +0000 UTC
> Timestamp: 2022-12-16 19:47:53.47415362 +0000 UTC
> SeverityText:
> SeverityNumber: Error(17)
> Body: Map({"Error":"example error","Message":"error context message"})
> Attributes:
>      -> testing: Bool(true)
> Trace ID:
> Span ID:
> Flags: 0
> 	{"kind": "exporter", "data_type": "logs", "name": "logging"}

## Stopping

```terminal
docker stop otel-collector
docker rm otel-collector
```
