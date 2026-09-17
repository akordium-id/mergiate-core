# M2M RPC Protocol (ConnectRPC & Pure gRPC)

`mergiate-core` menyediakan dual transport untuk integrasi antar-sistem Machine-to-Machine (M2M) berlatensi rendah:

1. **ConnectRPC** — berjalan langsung di atas port HTTP yang sama dengan Chi HTTP router bawaan (`APP_PORT`, default `:8080`). Mendukung:
   - Protokol Connect (HTTP POST + JSON/binary)
   - gRPC over HTTP/2 (`application/grpc`)
   - gRPC-Web (browser-friendly)
2. **Pure gRPC** (`google.golang.org/grpc`) — server gRPC standalone yang berjalan di port terpisah (`GRPC_PORT`, default `:50051`) dengan binary Protobuf.

---

## 1. Konfigurasi Environment

| Variable | Type | Default | Deskripsi |
|---|---|---|---|
| `GRPC_ENABLED` | bool | `true` | Mengaktifkan/menonaktifkan standalone gRPC server |
| `GRPC_PORT` | string | `50051` | Port listen untuk standalone gRPC server |
| `APP_PORT` | string | `8080` | Port HTTP (Chi REST + ConnectRPC) |

---

## 2. Services yang Tersedia

### PingService (`mergiate.v1.PingService`)
Endpoint diagnostik dan health check M2M (Public / Tanpa auth):
- `rpc Ping(PingRequest) returns (PingResponse)`

### Standard Health Service (`grpc.health.v1.Health`)
Standar health check gRPC (Public / Tanpa auth):
- `rpc Check(HealthCheckRequest) returns (HealthCheckResponse)`
- `rpc Watch(HealthCheckRequest) returns (stream HealthCheckResponse)`

### PartyService (`mergiate.v1.PartyService`)
Operasi entitas bisnis Party (Butuh Auth: JWT atau API Key):
- `rpc GetParty(GetPartyRequest) returns (GetPartyResponse)`
- `rpc CreateParty(CreatePartyRequest) returns (CreatePartyResponse)`
- `rpc ListParties(ListPartiesRequest) returns (ListPartiesResponse)`

---

## 3. Cara Akses

### A. ConnectRPC via cURL (HTTP JSON POST)
ConnectRPC memungkinkan query via cURL biasa ke port HTTP:

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

#### 2. Create Party (Dengan API Key)
```bash
curl -X POST http://localhost:8080/mergiate.v1.PartyService/CreateParty \
  -H "Content-Type: application/json" \
  -H "X-API-Key: mrg_live_xxxxxxxxxxxxxxxx" \
  -d '{
    "type": "organization",
    "name": "PT Akordium Teknologi",
    "code": "AKR",
    "initial_roles": ["customer"]
  }'
```

#### 3. Create Party (Dengan JWT Bearer Token)
```bash
curl -X POST http://localhost:8080/mergiate.v1.PartyService/CreateParty \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <jwt_token>" \
  -d '{
    "type": "person",
    "name": "Faiq Najib",
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

#### Health Check:
```bash
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
```

#### Party Service (dengan Metadata Auth):
```bash
grpcurl -plaintext \
  -H "x-api-key: mrg_live_xxxxxxxx" \
  -d '{"name": "Client A", "type": "organization"}' \
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

## 4. Re-generating Code dari Protobuf

Jika Anda menambah file `.proto` di direktori `proto/mergiate/v1/`:

Pastikan toolchain terinstall:
```bash
go install github.com/bufbuild/buf/cmd/buf@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
```

Lalu jalankan code generation:
```bash
buf generate
```
Kode baru akan otomatis tergenerate di dalam direktori `gen/mergiate/v1/`.
