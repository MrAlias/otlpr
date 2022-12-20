// Copyright 2022 Tyler Yahn (MrAlias)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/MrAlias/otlpr"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

const (
	lib    = "github.com/MrAlias/otlpr/example"
	libVer = "v0.1.0"
)

var targetPtr = flag.String("target", "127.0.0.1:4317", "OTLP target")

type App struct {
	tracer trace.Tracer
	logger logr.Logger
}

func NewApp(tracer trace.Tracer, logger logr.Logger) App {
	return App{tracer: tracer, logger: logger}
}

// Hello logs a greeting to user.
func (a App) Hello(ctx context.Context, user string) error {
	var span trace.Span
	ctx, span = a.tracer.Start(ctx, "Hello")
	defer span.End()

	if user == "" {
		span.SetStatus(codes.Error, "invalid user")
		return errors.New("no user name provided")
	}
	otlpr.WithContext(a.logger, ctx).Info("Hello!", "user", user)
	return nil
}

func setup(ctx context.Context, conn *grpc.ClientConn) (trace.Tracer, logr.Logger, error) {
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, logr.Discard(), err
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("example application"),
	)

	// Use a syncer for demo purposes only.
	tp := sdk.NewTracerProvider(sdk.WithSyncer(exp), sdk.WithResource(res))
	tracer := tp.Tracer(lib, trace.WithInstrumentationVersion(libVer))

	l := otlpr.NewWithOptions(conn, otlpr.Options{
		LogCaller:     otlpr.All,
		LogCallerFunc: true,
		Batcher:       otlpr.Batcher{Messages: 2, Timeout: 3 * time.Second},
	})
	l = otlpr.WithResource(l, res)
	scope := instrumentation.Scope{Name: lib, Version: libVer}
	l = otlpr.WithScope(l, scope)

	return tracer, l, nil
}

func main() {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	conn, err := grpc.DialContext(ctx, *targetPtr, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}

	tracer, logger, err := setup(ctx, conn)
	if err != nil {
		log.Fatal(err)
	}
	var span trace.Span
	ctx, span = tracer.Start(ctx, "main")
	defer span.End()

	app := NewApp(tracer, logger)
	for _, user := range []string{"alice", ""} {
		if err := app.Hello(ctx, user); err != nil {
			logger.Error(err, "failed to say hello", "user", user, "testing", true)
		}
	}
}
