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
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	lpb "go.opentelemetry.io/proto/otlp/logs/v1"
)

const noValue = "<no-value>"

// Defaults for Options.
const defaultMaxLogDepth = 16

type Options struct {
	// MaxLogDepth defines how many levels of nested fields (e.g. a struct that
	// contains a struct, etc.) to log. If this field is not specified, a
	// default value, 16, will be used.
	MaxLogDepth int
}

type Formatter struct {
	opts Options

	name       string
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
	out := make([]*cpb.KeyValue, 0, len(kvList)/2)
	for i := 0; i < len(kvList); i += 2 {
		kv := out[i/2]
		f.assignKeyValue(kv, kvList[i], kvList[i+1], 0)
	}
	return out
}

func (f Formatter) assignKeyValue(out *cpb.KeyValue, key, val interface{}, depth int) {
	switch k := key.(type) {
	case string:
		out.Key = k
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
	f.assignValue(out.Value, val, depth)
}

func (f Formatter) nonStringKey(k interface{}) string {
	return fmt.Sprintf("<non-string-key: %s>", f.snippet(k))
}

func (f Formatter) assignValue(out *cpb.AnyValue, val interface{}, depth int) {
	if depth > f.opts.MaxLogDepth {
		out.Value = &cpb.AnyValue_StringValue{
			StringValue: `"<max-log-depth-exceeded>"`,
		}
		return
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
		return
	case string:
		out.Value = &cpb.AnyValue_StringValue{StringValue: v}
		return
	case int:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
		return
	case int8:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
		return
	case int16:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
		return
	case int32:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
		return
	case int64:
		out.Value = &cpb.AnyValue_IntValue{IntValue: v}
		return
	case uint:
		f.assignUintVal(out, uint64(v))
		return
	case uint8:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
		return
	case uint16:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
		return
	case uint32:
		out.Value = &cpb.AnyValue_IntValue{IntValue: int64(v)}
		return
	case uintptr:
		f.assignUintVal(out, uint64(v))
		return
	case uint64:
		f.assignUintVal(out, uint64(v))
		return
	case float32:
		out.Value = &cpb.AnyValue_DoubleValue{DoubleValue: float64(v)}
		return
	case float64:
		out.Value = &cpb.AnyValue_DoubleValue{DoubleValue: v}
		return
	case complex64:
		out.Value = &cpb.AnyValue_StringValue{
			StringValue: `"` + strconv.FormatComplex(complex128(v), 'f', -1, 64) + `"`,
		}
		return
	case complex128:
		out.Value = &cpb.AnyValue_StringValue{
			StringValue: `"` + strconv.FormatComplex(v, 'f', -1, 64) + `"`,
		}
		return
	}

	t := reflect.TypeOf(val)
	if t == nil {
		// Empty value.
		return
	}
	v := reflect.ValueOf(val)
	switch t.Kind() {
	case reflect.Struct:
		n := t.NumField()
		kvs := make([]*cpb.KeyValue, n)
		total := n
		for i := 0; i < n; i++ {
			fld := t.Field(i)
			if fld.PkgPath != "" {
				// reflect says this field is only defined for non-exported
				// fields.
				total--
				continue
			}
			if !v.Field(i).CanInterface() {
				// reflect isn't clear exactly what this means, but we can't
				// use it.
				total--
				continue
			}
			var name string
			var omitempty bool
			if tag, found := fld.Tag.Lookup("json"); found {
				if tag == "-" {
					total--
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
				total--
				continue
			}
			kv := kvs[i]
			if fld.Anonymous && fld.Type.Kind() == reflect.Struct && name == "" {
				f.assignKeyValue(kv, fld.Type.String(), v.Field(i).Interface(), depth+1)
				continue
			}
			if name == "" {
				name = fld.Name
			}
			kv.Key = name
			f.assignValue(kv.Value, v.Field(i).Interface(), depth+1)
		}
		kvs = kvs[:total]
		out.Value = &cpb.AnyValue_KvlistValue{
			KvlistValue: &cpb.KeyValueList{Values: kvs},
		}
	case reflect.Slice, reflect.Array:
		a := make([]*cpb.AnyValue, v.Len())
		for i := 0; i < v.Len(); i++ {
			e := v.Index(i)
			f.assignValue(a[i], e.Interface(), depth+1)
		}
		out.Value = &cpb.AnyValue_ArrayValue{
			ArrayValue: &cpb.ArrayValue{Values: a},
		}
		return
	case reflect.Map:
		kvs := make([]*cpb.KeyValue, v.Len())
		iter := v.MapRange()
		var i int
		for iter.Next() {
			kv := kvs[i]
			k, v := iter.Key().Interface(), iter.Value().Interface()
			f.assignKeyValue(kv, k, v, depth+1)
			i++
		}
		out.Value = &cpb.AnyValue_KvlistValue{
			KvlistValue: &cpb.KeyValueList{Values: kvs},
		}
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			// Empty value.
			return
		}
		f.assignValue(out, v.Elem().Interface(), depth)
		return
	}

	out.Value = &cpb.AnyValue_StringValue{
		StringValue: fmt.Sprintf(`"<unhandled-%s>"`, t.Kind().String()),
	}
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

func (f Formatter) FormatInfo(level int, msg string, kvList []interface{}) *lpb.LogRecord {
	return &lpb.LogRecord{
		TimeUnixNano:   uint64(time.Now().UnixNano()),
		SeverityNumber: f.level(level),
		SeverityText:   strconv.Itoa(level),
		Body: &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{StringValue: msg},
		},
		Attributes: append(f.valuesAttr, f.attrs(kvList)...),
	}
}

func (f Formatter) FormatError(err error, msg string, kvList []interface{}) *lpb.LogRecord {
	return &lpb.LogRecord{
		TimeUnixNano:   uint64(time.Now().UnixNano()),
		SeverityNumber: lpb.SeverityNumber_SEVERITY_NUMBER_ERROR,
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
									StringValue: msg,
								},
							},
						},
					},
				},
			},
		},
		Attributes: append(f.valuesAttr, f.attrs(kvList)...),
	}
}

// AddName appends the specified name.
func (f *Formatter) AddName(name string) {
	if len(f.name) > 0 {
		f.name += "/"
	}
	f.name += name
}

func (f *Formatter) AddValues(kvList []interface{}) {
	// Three slice args forces a copy.
	n := len(f.values)
	f.values = append(f.values[:n:n], kvList...)

	// Pre-render values, so we don't have to do it on each Info/Error call.
	f.valuesAttr = f.attrs(kvList)
}

// AddCallDepth increases the number of stack-frames to skip when attributing
// the log line to a file and line.
func (f *Formatter) AddCallDepth(depth int) {
	f.depth += depth
}
