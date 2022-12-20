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
	"encoding"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	lpb "go.opentelemetry.io/proto/otlp/logs/v1"
	rpb "go.opentelemetry.io/proto/otlp/resource/v1"
)

// Based on https://pkg.go.dev/github.com/go-logr/logr/funcr.

const noValue = "<no-value>"

// Defaults for Options.
const defaultMaxLogDepth = 16

type Options struct {
	// LogCaller defines when to add a "caller" key to some or all log lines.
	// This has some overhead, so some users might not want it.
	LogCaller MessageClass

	// LogCallerFunc defines if the calling function name shoule be logged.
	// This has no effect if caller logging is not enabled (see
	// Options.LogCaller).
	LogCallerFunc bool

	// MaxLogDepth defines how many levels of nested fields (e.g. a struct that
	// contains a struct, etc.) to log. If this field is not specified, a
	// default value, 16, will be used.
	MaxLogDepth int
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

type Formatter struct {
	opts Options

	name       string
	spanCtx    trace.SpanContext
	depth      int
	values     []interface{}
	valuesAttr []*cpb.KeyValue
}

// NewFormatter returns a constructed Formatter.
func NewFormatter(opts Options) Formatter {
	if opts.MaxLogDepth == 0 {
		opts.MaxLogDepth = defaultMaxLogDepth
	}
	return Formatter{opts: opts}
}

// attrs returns kvList as encoded attributes.
func (f Formatter) attrs(kvList []interface{}) []*cpb.KeyValue {
	if len(kvList)%2 != 0 {
		kvList = append(kvList, noValue)
	}
	out := make([]*cpb.KeyValue, (len(kvList)+1)/2)
	for i := 0; i < len(kvList); i += 2 {
		out[i/2] = f.keyValue(kvList[i], kvList[i+1], 0)
	}
	return out
}

func (f Formatter) keyValue(key, val interface{}, depth int) *cpb.KeyValue {
	out := new(cpb.KeyValue)
	switch k := key.(type) {
	case string:
		out.Key = k
	case attribute.Key:
		out.Key = string(k)
	case encoding.TextMarshaler:
		txt, err := k.MarshalText()
		if err != nil {
			out.Key = fmt.Sprintf("<error-MarshalText: %s>", err.Error())
		} else {
			out.Key = string(txt)
		}
	default:
		out.Key = f.nonStringKey(key)
	}
	out.Value = f.value(val, depth)
	return out
}

func (f Formatter) nonStringKey(k interface{}) string {
	return fmt.Sprintf("<non-string-key: %s>", f.snippet(k))
}

func (f Formatter) value(val interface{}, depth int) *cpb.AnyValue {
	out := new(cpb.AnyValue)
	if depth > f.opts.MaxLogDepth {
		out.Value = &cpb.AnyValue_StringValue{
			StringValue: `"<max-log-depth-exceeded>"`,
		}
		return out
	}

	// Handle types that take full control of logging.
	if v, ok := val.(logr.Marshaler); ok {
		// Replace the value with what the type wants to get logged.
		// That then gets handled below via reflection.
		val = invokeMarshaler(v)
	}

	// Handle types that want to format themselves.
	switch v := val.(type) {
	case fmt.Stringer:
		val = invokeStringer(v)
	case error:
		val = invokeError(v)
	}

	switch v := val.(type) {
	case bool:
		out.Value = &cpb.AnyValue_BoolValue{BoolValue: v}
	case string:
		out.Value = &cpb.AnyValue_StringValue{StringValue: v}
	case int:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
	case int8:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
	case int16:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
	case int32:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
	case int64:
		out.Value = &cpb.AnyValue_IntValue{IntValue: v}
	case uint:
		f.assignUintVal(out, uint64(v))
	case uint8:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
	case uint16:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
	case uint32:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
	case uintptr:
		f.assignUintVal(out, uint64(v))
	case uint64:
		f.assignUintVal(out, uint64(v))
	case float32:
		out.Value = &cpb.AnyValue_DoubleValue{DoubleValue: float64(v)}
	case float64:
		out.Value = &cpb.AnyValue_DoubleValue{DoubleValue: v}
	case complex64:
		out.Value = &cpb.AnyValue_StringValue{
			StringValue: `"` + strconv.FormatComplex(complex128(v), 'f', -1, 64) + `"`,
		}
	case complex128:
		out.Value = &cpb.AnyValue_StringValue{
			StringValue: `"` + strconv.FormatComplex(v, 'f', -1, 64) + `"`,
		}
	}

	if out.Value != nil {
		return out
	}

	t := reflect.TypeOf(val)
	if t == nil {
		// Empty value.
		return out
	}
	v := reflect.ValueOf(val)
	switch t.Kind() {
	case reflect.Struct:
		n := t.NumField()
		kvs := make([]*cpb.KeyValue, 0, n)
		for i := 0; i < n; i++ {
			fld := t.Field(i)
			if fld.PkgPath != "" {
				// reflect says this field is only defined for non-exported
				// fields.
				continue
			}
			if !v.Field(i).CanInterface() {
				// reflect isn't clear exactly what this means, but we can't
				// use it.
				continue
			}
			var name string
			var omitempty bool
			if tag, found := fld.Tag.Lookup("json"); found {
				if tag == "-" {
					continue
				}
				if comma := strings.Index(tag, ","); comma != -1 {
					if n := tag[:comma]; n != "" {
						name = n
					}
					rest := tag[comma:]
					if strings.Contains(rest, ",omitempty,") || strings.HasSuffix(rest, ",omitempty") {
						omitempty = true
					}
				} else {
					name = tag
				}
			}
			if omitempty && isEmpty(v.Field(i)) {
				continue
			}
			if fld.Anonymous && fld.Type.Kind() == reflect.Struct && name == "" {
				kv := f.keyValue(fld.Type.String(), v.Field(i).Interface(), depth+1)
				kvs = append(kvs, kv)
				continue
			}
			if name == "" {
				name = fld.Name
			}
			kv := new(cpb.KeyValue)
			kv.Key = name
			kv.Value = f.value(v.Field(i).Interface(), depth+1)
			kvs = append(kvs, kv)
		}
		out.Value = &cpb.AnyValue_KvlistValue{
			KvlistValue: &cpb.KeyValueList{Values: kvs},
		}
	case reflect.Slice, reflect.Array:
		a := make([]*cpb.AnyValue, v.Len())
		for i := 0; i < v.Len(); i++ {
			e := v.Index(i)
			a[i] = f.value(e.Interface(), depth+1)
		}
		out.Value = &cpb.AnyValue_ArrayValue{
			ArrayValue: &cpb.ArrayValue{Values: a},
		}
	case reflect.Map:
		kvs := make([]*cpb.KeyValue, 0, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			k, v := iter.Key().Interface(), iter.Value().Interface()
			kvs = append(kvs, f.keyValue(k, v, depth+1))
		}
		out.Value = &cpb.AnyValue_KvlistValue{
			KvlistValue: &cpb.KeyValueList{Values: kvs},
		}
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			// Empty value.
			return out
		}
		return f.value(v.Elem().Interface(), depth)
	}

	if out.Value == nil {
		out.Value = &cpb.AnyValue_StringValue{
			StringValue: fmt.Sprintf(`"<unhandled-%s>"`, t.Kind().String()),
		}
	}
	return out
}

func isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

func (f Formatter) assignUintVal(out *cpb.AnyValue, val uint64) {
	const maxInt64 = ^uint64(0) >> 1
	if val > maxInt64 {
		out.Value = &cpb.AnyValue_StringValue{
			StringValue: strconv.FormatUint(val, 10),
		}
	} else {
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(val)}
	}
}

func invokeMarshaler(m logr.Marshaler) (ret interface{}) {
	defer func() {
		if r := recover(); r != nil {
			ret = fmt.Sprintf("<panic: %s>", r)
		}
	}()
	return m.MarshalLog()
}

func invokeStringer(s fmt.Stringer) (ret string) {
	defer func() {
		if r := recover(); r != nil {
			ret = fmt.Sprintf("<panic: %s>", r)
		}
	}()
	return s.String()
}

func invokeError(e error) (ret string) {
	defer func() {
		if r := recover(); r != nil {
			ret = fmt.Sprintf("<panic: %s>", r)
		}
	}()
	return e.Error()
}

// snippet produces a short snippet string of an arbitrary value.
func (f Formatter) snippet(v interface{}) string {
	const snipLen = 16

	snip := fmt.Sprintf("%#v", v)
	if len(snip) > snipLen {
		snip = snip[:snipLen]
	}
	return snip
}

func (f Formatter) level(l int) lpb.SeverityNumber {
	// In OpenTelemetry smaller numerical values in each range represent less
	// important (less severe) events. Larger numerical values in each range
	// represent more important (more severe) events.
	//
	// SeverityNumber range|Range name
	// --------------------|----------
	// 1-4                 |TRACE
	// 5-8                 |DEBUG
	// 9-12                |INFO
	// 13-16               |WARN
	// 17-20               |ERROR
	// 21-24               |FATAL
	//
	// Logr verbosity levels decrease in significance the greater the value.
	if l < 0 {
		l = 0
	}
	if l > int(lpb.SeverityNumber_SEVERITY_NUMBER_WARN4) {
		l = int(lpb.SeverityNumber_SEVERITY_NUMBER_WARN4)
	}
	return lpb.SeverityNumber(int(lpb.SeverityNumber_SEVERITY_NUMBER_WARN4) - l)
}

// Caller represents the original call site for a log line, after considering
// logr.Logger.WithCallDepth and logr.Logger.WithCallStackHelper.  The File and
// Line fields will always be provided, while the Func field is optional.
type Caller struct {
	// File is the basename of the file for this call site.
	File string `json:"file"`
	// Line is the line number in the file for this call site.
	Line int `json:"line"`
	// Func is the function name for this call site, or empty if
	// Options.LogCallerFunc is not enabled.
	Func string `json:"function,omitempty"`
}

func (f Formatter) caller() Caller {
	// +1 for this frame, +1 for Info/Error.
	pc, file, line, ok := runtime.Caller(f.depth + 2)
	if !ok {
		return Caller{"<unknown>", 0, ""}
	}
	fn := ""
	if f.opts.LogCallerFunc {
		if fp := runtime.FuncForPC(pc); fp != nil {
			fn = fp.Name()
		}
	}

	return Caller{filepath.Base(file), line, fn}
}

func (f Formatter) render(v lpb.SeverityNumber, body *cpb.AnyValue, kvList []interface{}) *lpb.LogRecord {
	out := &lpb.LogRecord{
		TimeUnixNano:   uint64(time.Now().UnixNano()),
		SeverityNumber: v,
		Body:           body,
		Attributes:     append(f.valuesAttr, f.attrs(kvList)...),
	}
	if f.spanCtx.IsValid() {
		tID := f.spanCtx.TraceID()
		out.TraceId = tID[:]

		sID := f.spanCtx.SpanID()
		out.SpanId = sID[:]

		out.Flags = uint32(f.spanCtx.TraceFlags())
	}
	return out
}

func (f Formatter) infoBody(msg string) *cpb.AnyValue {
	return &cpb.AnyValue{Value: &cpb.AnyValue_StringValue{StringValue: msg}}
}

func (f Formatter) errBody(err error, msg string) *cpb.AnyValue {
	return &cpb.AnyValue{
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
								StringValue: msg,
							},
						},
					},
				},
			},
		},
	}
}

// Init configures this Formatter from runtime info, such as the call depth
// imposed by logr itself.
func (f *Formatter) Init(info logr.RuntimeInfo) {
	f.depth += info.CallDepth
}

func (f Formatter) FormatInfo(level int, msg string, kvList []interface{}) *lpb.LogRecord {
	if policy := f.opts.LogCaller; policy == All || policy == Error {
		kvList = append(kvList, "caller", f.caller())
	}
	return f.render(f.level(level), f.infoBody(msg), kvList)
}

func (f Formatter) FormatError(err error, msg string, kvList []interface{}) *lpb.LogRecord {
	if policy := f.opts.LogCaller; policy == All || policy == Error {
		kvList = append(kvList, "caller", f.caller())
	}
	const v = lpb.SeverityNumber_SEVERITY_NUMBER_ERROR
	return f.render(v, f.errBody(err, msg), kvList)
}

func (f Formatter) FormatResource(res *resource.Resource) (string, *rpb.Resource) {
	iter := res.Iter()
	kvs := make([]*cpb.KeyValue, 0, iter.Len())
	for iter.Next() {
		attr := iter.Attribute()
		kvs = append(kvs, f.keyValue(attr.Key, attr.Value.AsInterface(), 0))
	}
	return res.SchemaURL(), &rpb.Resource{Attributes: kvs}
}

func (f Formatter) FormatScope(s instrumentation.Scope) (string, *cpb.InstrumentationScope) {
	out := &cpb.InstrumentationScope{
		Name:    s.Name,
		Version: s.Version,
	}
	return s.SchemaURL, out
}

// AddName appends the specified name.
func (f *Formatter) AddName(name string) {
	if len(f.name) > 0 {
		f.name += "/"
	}
	f.name += name
}

// AddContext adds log values for the span in ctx if it exists.
func (f *Formatter) AddContext(ctx context.Context) {
	f.spanCtx = trace.SpanContextFromContext(ctx)
}

func (f *Formatter) AddValues(kvList []interface{}) {
	// Three slice args forces a copy.
	n := len(f.values)
	f.values = append(f.values[:n:n], kvList...)

	// Pre-render values, so we don't have to do it on each Info/Error call.
	f.valuesAttr = f.attrs(f.values)
}

// AddCallDepth increases the number of stack-frames to skip when attributing
// the log line to a file and line.
func (f *Formatter) AddCallDepth(depth int) {
	f.depth += depth
}
