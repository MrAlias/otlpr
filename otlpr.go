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

// Package otlpr provides a github.com/go-logr/logr.Logger implementation that
// exports log records in the OpenTelemetry OTLP log format.
package otlpr

import (
	"context"

	"github.com/MrAlias/otlpr/internal"
	"github.com/go-logr/logr"
	collpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	lpb "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/grpc"
)

// New returns a new logr Logger that will export logs over conn using OTLP.
// The conn is expected to be ready to use when passed. If conn is nil a
// discard logger is returned.
func New(conn *grpc.ClientConn) logr.Logger {
	if conn == nil {
		return logr.Discard()
	}

	l := &logger{
		exp:       newExporter(conn),
		formatter: internal.NewFormatter(internal.Options{}),
	}
	return logr.New(l)
}

type logger struct {
	exp *exporter

	formatter internal.Formatter
	level     int
}

var _ logr.LogSink = &logger{}

func (l *logger) Init(logr.RuntimeInfo) {}

func (l *logger) Enabled(level int) bool {
	return level >= l.level
}

func (l *logger) Info(level int, msg string, keysAndValues ...interface{}) {
	l.exp.enqueue(l.formatter.FormatInfo(level, msg, keysAndValues))
}

func (l *logger) Error(err error, msg string, keysAndValues ...interface{}) {
	l.exp.enqueue(l.formatter.FormatError(err, msg, keysAndValues))
}

func (l *logger) WithValues(keysAndValues ...interface{}) logr.LogSink {
	l.formatter.AddValues(keysAndValues)
	return l
}

func (l *logger) WithName(name string) logr.LogSink {
	l.formatter.AddName(name)
	return l
}

type exporter struct {
	client collpb.LogsServiceClient
}

func newExporter(conn *grpc.ClientConn) *exporter {
	return &exporter{client: collpb.NewLogsServiceClient(conn)}
}

func (e *exporter) enqueue(msg *lpb.LogRecord) {
	// TODO: handle batching.
	_, _ = e.client.Export(context.Background(), &collpb.ExportLogsServiceRequest{
		ResourceLogs: []*lpb.ResourceLogs{
			{
				ScopeLogs: []*lpb.ScopeLogs{
					{
						LogRecords: []*lpb.LogRecord{
							msg,
						},
					},
				},
			},
		},
	})
	// TODO: handle partial success response.
	// TODO: handle returned error (log it?).
}
