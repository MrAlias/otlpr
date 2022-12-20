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

	collpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	lpb "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/grpc"
)

type exporter struct {
	client collpb.LogsServiceClient
}

func newExporter(conn *grpc.ClientConn) *exporter {
	return &exporter{client: collpb.NewLogsServiceClient(conn)}
}

func (e *exporter) enqueue(rl *lpb.ResourceLogs) {
	// TODO: handle batching.
	_, _ = e.client.Export(context.Background(), &collpb.ExportLogsServiceRequest{
		ResourceLogs: []*lpb.ResourceLogs{rl},
	})
	// TODO: handle partial success response.
	// TODO: handle returned error (log it?).
}
