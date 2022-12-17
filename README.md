# otlpr

A logr implementation using OTLP

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
