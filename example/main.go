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

	"github.com/MrAlias/otlpr"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

var targetPtr = flag.String("target", "127.0.0.1:4317", "OTLP target")

func tracerProvider(ctx context.Context, conn *grpc.ClientConn) (trace.TracerProvider, error) {
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}
	// Use a syncer for demo purposes only.
	return sdk.NewTracerProvider(sdk.WithSyncer(exp)), nil
}

func main() {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	conn, err := grpc.DialContext(ctx, *targetPtr, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}

	tp, err := tracerProvider(ctx, conn)
	if err != nil {
		log.Fatal(err)
	}

	var span trace.Span
	ctx, span = tp.Tracer("github.com/MrAlias/otlpr/example").Start(ctx, "main")
	defer span.End()

	l := otlpr.NewWithOptions(conn, otlpr.Options{
		LogCaller:     otlpr.All,
		LogCallerFunc: true,
	})
	l = otlpr.WithContext(l, ctx)
	l.Info("information message", "function", "main")

	err = errors.New("example error")
	l.Error(err, "error context message", "testing", true)
}
