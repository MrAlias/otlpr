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
	"github.com/MrAlias/otlpr/internal"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
)

func New(conn *grpc.ClientConn) logr.Logger {
	// FIXME: handle nil conn.
	l := &logger{
		conn:      conn,
		formatter: internal.NewFormatter(internal.Options{}),
	}
	return logr.New(l)
}

type logger struct {
	conn *grpc.ClientConn

	formatter internal.Formatter
	level     int
}

var _ logr.LogSink = &logger{}

func (l *logger) Init(logr.RuntimeInfo) {}

func (l *logger) Enabled(level int) bool {
	return level >= l.level
}

func (l *logger) Info(level int, msg string, keysAndValues ...interface{})
func (l *logger) Error(err error, msg string, keysAndValues ...interface{})
func (l *logger) WithValues(keysAndValues ...interface{}) logr.LogSink
func (l *logger) WithName(name string) logr.LogSink
