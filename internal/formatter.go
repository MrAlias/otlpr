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
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	lpb "go.opentelemetry.io/proto/otlp/logs/v1"
)

const noValue = "<no-value>"

type Formatter struct {
	name       string
	depth      int
	values     []interface{}
	valuesAttr []*cpb.KeyValue
}

// attrs returns kvList as encoded attributes.
func (f Formatter) attrs(kvList []interface{}) []*cpb.KeyValue {
	if len(kvList)%2 != 0 {
		kvList = append(kvList, noValue)
	}
	out := make([]*cpb.KeyValue, 0, len(kvList)/2)
	for i := 0; i < len(kvList); i += 2 {
		kv := out[i/2]
		var ok bool
		kv.Key, ok = kvList[i].(string)
		if !ok {
			kv.Key = f.nonStringKey(kvList[i])
		}
		f.assignValue(kv.Value, kvList[i+1])
	}
	return out
}

func (f Formatter) nonStringKey(k interface{}) string {
	return fmt.Sprintf("<non-string-key: %s>", f.snippet(k))
}

func (f Formatter) assignValue(out *cpb.AnyValue, val interface{}) {
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

	// FIXME: use reflect from here on.
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

func (f Formatter) FormatInfo(level int, msg string, kvList []interface{}) *lpb.LogRecord {
	// FIXME: implement.
	return nil
}

func (f Formatter) FormatError(err error, msg string, kvList []interface{}) *lpb.LogRecord {
	// FIXME: implement.
	return nil
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
