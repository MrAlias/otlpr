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
	"google.golang.org/grpc"
)

// New returns a new logr Logger that will export logs over conn using OTLP.
// The conn is expected to be ready to use when passed. If conn is nil a
// discard logger is returned.
func New(conn *grpc.ClientConn) logr.Logger {
	return NewWithOptions(conn, Options{})
}

// NewWithOptions returns a new logr Logger that will export logs over conn using OTLP. See New for details.
func NewWithOptions(conn *grpc.ClientConn, opts Options) logr.Logger {
	if conn == nil {
		return logr.Discard()
	}

	if opts.Depth < 0 {
		opts.Depth = 0
	}

	fopts := internal.Options{
		LogCaller:     internal.MessageClass(opts.LogCaller),
		LogCallerFunc: opts.LogCallerFunc,
	}

	l := &logSink{
		exp:       newExporter(conn),
		formatter: internal.NewFormatter(fopts),
	}

	// For skipping both our own logSink.Info/Error and logr's.
	l.formatter.AddCallDepth(2 + opts.Depth)

	return logr.New(l)
}

// Options carries parameters which influence the way logs are generated.
type Options struct {
	// Depth biases the assumed number of call frames to the "true" caller.
	// This is useful when the calling code calls a function which then calls
	// stdr (e.g. a logging shim to another API).  Values less than zero will
	// be treated as zero.
	Depth int

	// LogCaller tells otlpr to add a "caller" key to some or all log lines.
	LogCaller MessageClass

	// LogCallerFunc tells otlpr to also log the calling function name. This
	// has no effect if caller logging is not enabled (see Options.LogCaller).
	LogCallerFunc bool
}

// MessageClass indicates which category or categories of messages to consider.
type MessageClass int

const (
	// None ignores all message classes.
	None MessageClass = iota
	// All considers all message classes.
	All
	// Info only considers info messages.
	Info
	// Error only considers error messages.
	Error
)

type logSink struct {
	exp *exporter

	formatter internal.Formatter
	level     int
}

var _ logr.LogSink = &logSink{}

func (l *logSink) Init(logr.RuntimeInfo) {}

func (l *logSink) Enabled(level int) bool {
	return level >= l.level
}

func (l *logSink) Info(level int, msg string, keysAndValues ...interface{}) {
	l.exp.enqueue(l.formatter.FormatInfo(level, msg, keysAndValues))
}

func (l *logSink) Error(err error, msg string, keysAndValues ...interface{}) {
	l.exp.enqueue(l.formatter.FormatError(err, msg, keysAndValues))
}

func (l *logSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	l.formatter.AddValues(keysAndValues)
	return l
}

func (l *logSink) WithName(name string) logr.LogSink {
	l.formatter.AddName(name)
	return l
}

func (l *logSink) WithContext(ctx context.Context) logr.LogSink {
	l.formatter.AddContext(ctx)
	return l
}

// WithContext returns an updated logger that will log information about any
// span in ctx if one exists with each log message. It does nothing for loggers
// where the sink doesn't support a context.
func WithContext(l logr.Logger, ctx context.Context) logr.Logger {
	if ls, ok := l.GetSink().(*logSink); ok {
		l = l.WithSink(ls.WithContext(ctx))
	}
	return l
}
