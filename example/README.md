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

## Stopping

```terminal
docker stop otel-collector
docker rm otel-collector
```
