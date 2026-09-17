# M2M RPC Protocol (ConnectRPC & Pure gRPC)

`mergiate-core` provides dual transport layers for low-latency Machine-to-Machine (M2M) system integration:

1. **ConnectRPC** — runs alongside the default Chi HTTP router on the same application port (`APP_PORT`, default `:8080`). It supports:
   - Connect protocol (HTTP POST with JSON or binary Protobuf)
   - Standard gRPC over HTTP/2 (`application/grpc`)
   - gRPC-Web (browser and edge-friendly)
2. **Pure gRPC** (`google.golang.org/grpc`) — a standalone gRPC server running on a dedicated port (`GRPC_PORT`, default `:50051`) using native binary Protobuf.

---

## 1. Environment Configuration

| Variable | Type | Default | Description |
|---|---|---|---|
| `GRPC_ENABLED` | bool | `true` | Enables or disables the standalone gRPC server |
| `GRPC_PORT` | string | `50051` | Listening port for standalone gRPC server |
| `APP_PORT` | string | `8080` | Listening port for HTTP server (Chi REST + ConnectRPC) |

---

## 2. Available Services

### PingService (`mergiate.v1.PingService`)
Diagnostic and connectivity ping endpoint (Public / Unauthenticated):
- `rpc Ping(PingRequest) returns (PingResponse)`

### Standard Health Service (`grpc.health.v1.Health`)
Standard gRPC health checking protocol (Public / Unauthenticated):
- `rpc Check(HealthCheckRequest) returns (HealthCheckResponse)`
- `rpc Watch(HealthCheckRequest) returns (stream HealthCheckResponse)`

### PartyService (`mergiate.v1.PartyService`)
Business aggregate operations for Parties (Requires Authentication: JWT Bearer or M2M API Key):
- `rpc GetParty(GetPartyRequest) returns (GetPartyResponse)`
- `rpc CreateParty(CreatePartyRequest) returns (CreatePartyResponse)`
- `rpc ListParties(ListPartiesRequest) returns (ListPartiesResponse)`

---

## 3. Usage & Examples

### A. ConnectRPC via cURL (HTTP JSON POST)
ConnectRPC allows calling RPC procedures using standard HTTP POST requests:

#### 1. Ping
```bash
curl -X POST http://localhost:8080/mergiate.v1.PingService/Ping \
  -H "Content-Type: application/json" \
  -d '{"message": "hello mergiate"}'
```

Response:
```json
{
  "message": "hello mergiate",
  "version": "mergiate-core",
  "timestamp": 1773995832
}
```

#### 2. Create Party (With M2M API Key)
```bash
curl -X POST http://localhost:8080/mergiate.v1.PartyService/CreateParty \
  -H "Content-Type: application/json" \
  -H "X-API-Key: mrg_live_xxxxxxxxxxxxxxxx" \
  -d '{
    "type": "organization",
    "name": "Acme Technologies",
    "code": "ACM",
    "initial_roles": ["customer"]
  }'
```

#### 3. Create Party (With User JWT Bearer Token)
```bash
curl -X POST http://localhost:8080/mergiate.v1.PartyService/CreateParty \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <jwt_token>" \
  -d '{
    "type": "person",
    "name": "John Doe",
    "initial_roles": ["employee"]
  }'
```

---

### B. Pure gRPC via `grpcurl`

#### Ping (Port 50051):
```bash
grpcurl -plaintext -d '{"message": "test ping"}' \
  localhost:50051 mergiate.v1.PingService/Ping
```

#### Standard Health Check:
```bash
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
```

#### Party Service (with Auth Metadata):
```bash
grpcurl -plaintext \
  -H "x-api-key: mrg_live_xxxxxxxx" \
  -d '{"name": "Acme Corp", "type": "organization"}' \
  localhost:50051 mergiate.v1.PartyService/CreateParty
```

---

### C. Go Client (ConnectRPC)

```go
package main

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	mergiatev1 "github.com/akordium-id/mergiate-core/gen/mergiate/v1"
	"github.com/akordium-id/mergiate-core/gen/mergiate/v1/mergiatev1connect"
)

func main() {
	client := mergiatev1connect.NewPingServiceClient(
		http.DefaultClient,
		"http://localhost:8080",
	)

	req := connect.NewRequest(&mergiatev1.PingRequest{
		Message: "Hello from Go!",
	})

	res, err := client.Ping(context.Background(), req)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Ping response: %s (version: %s)\n", res.Msg.Message, res.Msg.Version)
}
```

---

### D. Go Client (Pure gRPC)

```go
package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	mergiatev1 "github.com/akordium-id/mergiate-core/gen/mergiate/v1"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := mergiatev1.NewPartyServiceClient(conn)

	// Inject API key via gRPC metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-api-key", "mrg_live_xxxx")

	resp, err := client.CreateParty(ctx, &mergiatev1.CreatePartyRequest{
		Name: "New Vendor",
		Type: "organization",
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Party created: ID=%s, Name=%s\n", resp.Party.Id, resp.Party.Name)
}
```

---

## 4. Re-generating Code from Protobuf

If you update or add `.proto` schemas in the `proto/mergiate/v1/` directory:

Ensure required plugins are installed:
```bash
go install github.com/bufbuild/buf/cmd/buf@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
```

Execute code generation:
```bash
buf generate
```
All stubs will be generated in `gen/mergiate/v1/`.
