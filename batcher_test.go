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

package otlpr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	lpb "go.opentelemetry.io/proto/otlp/logs/v1"
)

func expFn(chSize int) (<-chan []*lpb.LogRecord, exportFunc) {
	c := make(chan []*lpb.LogRecord, chSize)
	f := func(in []*lpb.LogRecord) { c <- in }
	return c, f
}

func TestChunk(t *testing.T) {
	c, f := expFn(3)
	f = chunk(10, f)
	f(make([]*lpb.LogRecord, 25))

	expectedLen := []int{10, 10, 5}
	for i, n := range expectedLen {
		got := <-c
		assert.Lenf(t, got, n, "chunk %d", i)
	}

	select {
	case v := <-c:
		assert.Failf(t, "extra chunk", "length: %d", len(v))
	default:
	}
}

func assertNoExport(t *testing.T, c <-chan []*lpb.LogRecord) {
	t.Helper()
	select {
	case got := <-c:
		assert.Failf(t, "unexpected export", "%#v", got)
	default:
	}
}

func assertExport(t *testing.T, c <-chan []*lpb.LogRecord, n int) {
	t.Helper()
	select {
	case got := <-c:
		assert.Len(t, got, n)
	default:
		assert.Fail(t, "missing export")
	}
}

func TestMessages(t *testing.T) {
	c, f := expFn(1)
	b := Batcher{Messages: 3}.start(f)
	msg := &lpb.LogRecord{}

	b.Append(msg)
	assertNoExport(t, c)

	b.Append(msg)
	assertNoExport(t, c)

	b.Append(msg)
	assertExport(t, c, 3)
}

func TestTimeout(t *testing.T) {
	c, f := expFn(1)
	b := Batcher{Messages: 2048, Timeout: time.Nanosecond}.start(f)
	msg := &lpb.LogRecord{}

	b.Append(msg)
	select {
	case got := <-c:
		assert.Len(t, got, 1)
	case <-time.After(3 * time.Second):
		assert.Fail(t, "missing export")
	}
}
