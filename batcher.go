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
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	lpb "go.opentelemetry.io/proto/otlp/logs/v1"
)

type Batcher struct {
	// Messages is the maximum number of messages to queue. Once this many
	// messages have been queued the Batcher will export the queue.
	//
	// If Messages is less than or equal to one it will export messages as they
	// arrive, no batching will be perfomed.
	Messages uint64
	// Timeout is the maximum time to wait for a full queue. Once this much
	// time has elapsed since the last export or start the Batcher will export
	// the queue.
	//
	// If Timeout is less than or equal to zero the Batcher will never export
	// based on queue staleness.
	Timeout time.Duration
	// ExportN is the maximum number of messages included in an export.
	//
	// For values less than or equal to zero the Batcher will export the whole
	// queue in a single export.
	ExportN int
}

type exportFunc func([]*lpb.LogRecord)

func chunk(n int, f exportFunc) exportFunc {
	return func(lr []*lpb.LogRecord) {
		for i, j := 0, n; i < len(lr); i, j = i+n, j+n {
			if j > len(lr) {
				j = len(lr)
			}
			f(lr[i:j])
		}
	}
}

func (b Batcher) start(expFn exportFunc) *batcher {
	if b.Messages == 0 {
		b.Messages = 1
	}
	if expFn == nil {
		expFn = func([]*lpb.LogRecord) {}
	}
	return newBatcher(b, expFn)
}

type batcher struct {
	export exportFunc

	timeout  time.Duration
	activeMu sync.Mutex
	active   *batch
	appender atomic.Value // func(*lpb.LogRecord)

	wg           sync.WaitGroup
	cancel       context.CancelFunc
	shutdownOnce sync.Once
}

func newBatcher(conf Batcher, expFn exportFunc) *batcher {
	if conf.ExportN > 0 {
		expFn = chunk(conf.ExportN, expFn)
	}

	b := &batcher{timeout: conf.Timeout, export: expFn}

	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.appender.Store(b.append)
	b.active = newBatch(conf.Messages)
	if conf.Timeout > 0 {
		go b.poll(ctx)
	}

	runtime.SetFinalizer(b, (*batcher).Shutdown)
	return b
}

func (b *batcher) poll(parent context.Context) {
	b.wg.Add(1)
	defer b.wg.Done()

	timestamp := time.Now()
	reset := func() time.Time {
		b.activeMu.Lock()
		defer b.activeMu.Unlock()

		if ts := b.active.Timestamp(); ts.IsZero() || ts.After(timestamp) {
			return ts
		}

		b.export(b.active.Flush())
		return time.Now()
	}

	for {
		deadline := timestamp.Add(b.timeout)
		ctx, cancel := context.WithDeadline(parent, deadline)
		<-ctx.Done()

		// Most likely unneeded, but it ensure no future leak.
		cancel()

		switch ctx.Err() {
		case context.DeadlineExceeded:
			timestamp = reset()
		case nil:
			// This shouldn't happen. Restart if it does.
		default:
			return
		}
	}
}

func (b *batcher) Append(msg *lpb.LogRecord) {
	if msg == nil {
		return
	}
	b.appender.Load().(func(*lpb.LogRecord))(msg)
}

func (b *batcher) append(msg *lpb.LogRecord) {
	b.activeMu.Lock()
	defer b.activeMu.Unlock()
	if complete := b.active.Append(msg); complete {
		b.export(b.active.Flush())
	}
}

func (b *batcher) Shutdown() { b.shutdownOnce.Do(b.shutdown) }

func (b *batcher) shutdown() {
	b.appender.Store(func(*lpb.LogRecord) {})

	// Acquire the lock after switching the appender to both ensure no active
	// calls to Append are in progress and guard the active batch.
	b.activeMu.Lock()
	b.export(b.active.Flush())
	b.activeMu.Unlock()

	// Close poller.
	b.cancel()
	done := make(chan struct{}, 1)
	go func() {
		b.wg.Wait()
		done <- struct{}{}
		close(done)
	}()
	<-done
}

type batch []*lpb.LogRecord

func newBatch(n uint64) *batch {
	b := make(batch, 0, int(n))
	return &b
}

func (b *batch) Len() int {
	return len(*b)
}

func (b *batch) Timestamp() time.Time {
	if b.Len() == 0 {
		return time.Time{}
	}
	return time.Unix(0, int64((*b)[0].GetTimeUnixNano()))
}

func (b *batch) Append(msg *lpb.LogRecord) bool {
	*b = append(*b, msg)
	return b.Len() == cap(*b)
}

func (b *batch) Flush() []*lpb.LogRecord {
	cp := make(batch, b.Len())
	copy(cp, *b)
	*b = (*b)[:0]
	return cp
}
