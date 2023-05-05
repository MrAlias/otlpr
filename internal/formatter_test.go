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

package internal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	lpb "go.opentelemetry.io/proto/otlp/logs/v1"
	rpb "go.opentelemetry.io/proto/otlp/resource/v1"
)

var (
	// Sat Jan 01 2000 00:00:00 GMT+0000.
	staticTime    = time.Unix(946684800, 0)
	staticNowFunc = func() time.Time { return staticTime }
	// Pass to t.Cleanup to override the now function with staticNowFunc and
	// revert once the test completes. E.g. t.Cleanup(mockTime(now)).
	mockTime = func(orig func() time.Time) (cleanup func()) {
		now = staticNowFunc
		return func() { now = orig }
	}
)

type marshaler string

func (m marshaler) MarshalText() ([]byte, error) { return []byte(m), nil }

func (m marshaler) MarshalLog() interface{} { return string(m) }

type stringer string

func (s stringer) String() string { return string(s) }

func TestFormatterAttrs(t *testing.T) {
	input := []interface{}{
		"string key", "",
		attribute.Key("attr key"), "",
		marshaler("marshal key"), "",
		1, "",
		"logr Marshaler", marshaler("logr"),
		"stringer", stringer("stringer"),
		"error", errors.New("error"),
		"bool", true,
		"int", int(2),
		"int8", int8(2),
		"int16", int16(2),
		"int32", int32(2),
		"int64", int64(2),
		"uint", uint(2),
		"uint8", uint8(2),
		"uint16", uint16(2),
		"uint32", uint32(2),
		"uintptr", uintptr(2),
		"uint64", uint64(2),
		"float32", float32(2),
		"float64", float64(2),
		"complex64", complex64(2),
		"complex128", complex128(2),
		"struct", struct{ Value string }{Value: "struct"},
		"slice", []int{-1, 0},
		"map", map[string]int{"one": 1},
		"ptr", new(int),
		"nil", nil,
	}

	want := []*cpb.KeyValue{
		{Key: "string key", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: ""},
		}},
		{Key: "attr key", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: ""},
		}},
		{Key: "marshal key", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: ""},
		}},
		{Key: "<non-string-key: 1>", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: ""},
		}},
		{Key: "logr Marshaler", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: "logr"},
		}},
		{Key: "stringer", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: "stringer"},
		}},
		{Key: "error", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: "error"},
		}},
		{Key: "bool", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_BoolValue{BoolValue: true},
		}},
		{Key: "int", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 2},
		}},
		{Key: "int8", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 2},
		}},
		{Key: "int16", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 2},
		}},
		{Key: "int32", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 2},
		}},
		{Key: "int64", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 2},
		}},
		{Key: "uint", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 2},
		}},
		{Key: "uint8", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 2},
		}},
		{Key: "uint16", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 2},
		}},
		{Key: "uint32", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 2},
		}},
		{Key: "uintptr", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 2},
		}},
		{Key: "uint64", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 2},
		}},
		{Key: "float32", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_DoubleValue{DoubleValue: 2},
		}},
		{Key: "float64", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_DoubleValue{DoubleValue: 2},
		}},
		{Key: "complex64", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: `"(2+0i)"`},
		}},
		{Key: "complex128", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: `"(2+0i)"`},
		}},
		{Key: "struct", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_KvlistValue{
				KvlistValue: &cpb.KeyValueList{
					Values: []*cpb.KeyValue{
						{
							Key: "Value",
							Value: &cpb.AnyValue{
								Value: &cpb.AnyValue_StringValue{StringValue: "struct"},
							},
						},
					},
				},
			},
		}},
		{Key: "slice", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_ArrayValue{
				ArrayValue: &cpb.ArrayValue{
					Values: []*cpb.AnyValue{
						{Value: &cpb.AnyValue_IntValue{IntValue: -1}},
						{Value: &cpb.AnyValue_IntValue{IntValue: 0}},
					},
				},
			},
		}},
		{Key: "map", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_KvlistValue{
				KvlistValue: &cpb.KeyValueList{
					Values: []*cpb.KeyValue{
						{
							Key: "one",
							Value: &cpb.AnyValue{
								Value: &cpb.AnyValue_IntValue{IntValue: 1},
							},
						},
					},
				},
			},
		}},
		{Key: "ptr", Value: &cpb.AnyValue{
			Value: &cpb.AnyValue_IntValue{IntValue: 0},
		}},
		{Key: "nil", Value: &cpb.AnyValue{}},
	}

	f := NewFormatter(Options{})
	assert.Equal(t, want, f.attrs(input))
}

func TestFormatterFormatInfo(t *testing.T) {
	t.Cleanup(mockTime(now))

	f := NewFormatter(Options{})
	got := f.FormatInfo(2, "message", []interface{}{"key", "value"})
	want := &lpb.LogRecord{
		TimeUnixNano:   uint64(staticTime.UnixNano()),
		SeverityNumber: 14,
		Body: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: "message"},
		},
		Attributes: []*cpb.KeyValue{
			{Key: "key", Value: &cpb.AnyValue{
				Value: &cpb.AnyValue_StringValue{StringValue: "value"},
			}},
		},
	}
	assert.Equal(t, want, got)
}

func TestFormatterFormatError(t *testing.T) {
	t.Cleanup(mockTime(now))

	f := NewFormatter(Options{})
	err := errors.New("error msg")
	got := f.FormatError(err, "message", []interface{}{"key", "value"})
	want := &lpb.LogRecord{
		TimeUnixNano:   uint64(staticTime.UnixNano()),
		SeverityNumber: 17,
		Body: &cpb.AnyValue{
			Value: &cpb.AnyValue_KvlistValue{
				KvlistValue: &cpb.KeyValueList{
					Values: []*cpb.KeyValue{
						{
							Key: "Error",
							Value: &cpb.AnyValue{
								Value: &cpb.AnyValue_StringValue{
									StringValue: err.Error(),
								},
							},
						},
						{
							Key: "Message",
							Value: &cpb.AnyValue{
								Value: &cpb.AnyValue_StringValue{
									StringValue: "message",
								},
							},
						},
					},
				},
			},
		},
		Attributes: []*cpb.KeyValue{
			{Key: "key", Value: &cpb.AnyValue{
				Value: &cpb.AnyValue_StringValue{StringValue: "value"},
			}},
		},
	}
	assert.Equal(t, want, got)
}

func TestFormatterFormatResource(t *testing.T) {
	f := NewFormatter(Options{})

	schemaURL := "http://opentelemetry.io"
	res := resource.NewWithAttributes(
		schemaURL,
		attribute.String("service.name", "test"),
	)

	gotURL, gotRes := f.FormatResource(res)

	assert.Equal(t, schemaURL, gotURL)

	wantRes := &rpb.Resource{
		Attributes: []*cpb.KeyValue{
			{
				Key: "service.name",
				Value: &cpb.AnyValue{
					Value: &cpb.AnyValue_StringValue{StringValue: "test"},
				},
			},
		},
	}
	assert.Equal(t, wantRes, gotRes)
}

func TestFormatterFormatScope(t *testing.T) {
	f := NewFormatter(Options{})

	schemaURL := "http://opentelemetry.io"
	scope := instrumentation.Scope{
		Name:      "name",
		Version:   "version",
		SchemaURL: schemaURL,
	}

	gotURL, gotS := f.FormatScope(scope)

	assert.Equal(t, schemaURL, gotURL)

	wantS := &cpb.InstrumentationScope{
		Name:    "name",
		Version: "version",
	}
	assert.Equal(t, wantS, gotS)
}

func TestFormatterAddContext(t *testing.T) {
	t.Cleanup(mockTime(now))

	f := NewFormatter(Options{})
	tID, sID := trace.TraceID{1}, trace.SpanID{1}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tID,
		SpanID:     sID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	f.AddContext(ctx)

	got := f.FormatInfo(0, "message", nil)
	want := &lpb.LogRecord{
		TimeUnixNano:   uint64(staticTime.UnixNano()),
		SeverityNumber: 16,
		Body: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: "message"},
		},
		Flags:   1,
		TraceId: []byte(tID[:]),
		SpanId:  []byte(sID[:]),
	}
	assert.Equal(t, want, got)
}

func TestFormatterAddValues(t *testing.T) {
	t.Cleanup(mockTime(now))

	f := NewFormatter(Options{})
	f.AddValues([]interface{}{"one", 1})
	got := f.FormatInfo(0, "message", []interface{}{"two", 2})
	want := &lpb.LogRecord{
		TimeUnixNano:   uint64(staticTime.UnixNano()),
		SeverityNumber: 16,
		Body: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: "message"},
		},
		Attributes: []*cpb.KeyValue{
			{Key: "one", Value: &cpb.AnyValue{
				Value: &cpb.AnyValue_IntValue{IntValue: 1},
			}},
			{Key: "two", Value: &cpb.AnyValue{
				Value: &cpb.AnyValue_IntValue{IntValue: 2},
			}},
		},
	}
	assert.Equal(t, want, got)
}
