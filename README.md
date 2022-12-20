# otlpr

[![Go Reference](https://pkg.go.dev/badge/github.com/MrAlias/otlpr.svg)](https://pkg.go.dev/github.com/MrAlias/otlpr)

A logr implementation using OTLP

:construction: This repository is a work in progresss and not production ready.

## Getting Started

A working gRPC connection to an OTLP endpoint is needed to setup the logger.

```go
conn, _ := grpc.DialContext(ctx, otlpTarget)
```

Create a [`logr.Logger`] with this connection.

```go
logger := otlpr.New(conn)
```

See the [example] for a working example application.

## Batching

By default the logger will export single log messages as they are received.
A `Batcher` can be used to change this behavior.

```go
opts := otlpr.Options{
	Batcher: otlpr.Batcher{
		// Only queue at most 100 messages.
		Messages: 100,
		// Only wait 3 seconds for the queue to fill.
		Timeout: 3 * time.Second,
	},
}
logger := otlpr.NewWithOptions(conn, opts)
```

## Annotating Span Context

OTLP is able to associate span context with log messages.
Use the `WithContext` function to associate a `context.Context` that contains an active span with all logs the logger creates.

```go
logger = otlpr.WithContext(logger, ctx)
```

The function can also be used to clear any span context from the logger.

```go
logger = otlpr.WithContext(logger, context.Background())
```

[`logr.Logger`]: https://pkg.go.dev/github.com/go-logr/logr#Logger
[example]: ./example/

## Adding a Resource

The system a log message is produced in can be described with a [`Resource`].
Use the `WithResource` function to include this information with the exported OTLP data.

```go
logger = otlpr.WithResource(logger, resource)
```

The function can also be used to clear any resource from the logger.

```go
logger = otlpr.WithResource(logger, nil)
```

## Adding Scope

The portion of a system a log message is produced in can be described with [`Scope`].
Use the `WithScope` function to include this information with the exported OTLP data.

```go
logger = otlpr.WithScope(logger, resource)
```

The function can also be used to clear any scope from the logger.

```go
logger = otlpr.WithScope(logger, instrumentation.Scope{})
```

[`logr.Logger`]: https://pkg.go.dev/github.com/go-logr/logr#Logger
[example]: ./example/
[`Resource`]: https://pkg.go.dev/go.opentelemetry.io/otel/sdk/resource#Resource
[`Scope`]: https://pkg.go.dev/go.opentelemetry.io/otel/sdk/instrumentation#Scope
