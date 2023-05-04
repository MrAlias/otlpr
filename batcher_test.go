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

	"github.com/stretchr/testify/assert"
	lpb "go.opentelemetry.io/proto/otlp/logs/v1"
)

func TestChunk(t *testing.T) {
	c := make(chan []*lpb.LogRecord, 3)
	f := func(in []*lpb.LogRecord) { c <- in }
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
