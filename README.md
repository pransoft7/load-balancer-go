# L4 Proxy based Load Balancer in Go

A TCP load balancer built from scratch as a way to learn distributed systems concepts and exploring the underlying architecture of a load balancer.

This isn't production-ready, but an exercise to make design decisions, understand the trade-offs, and implement it in code.

---

## What it does

This is a Layer 4 (TCP) proxy. It sits in front of a pool of backend servers, accepts raw TCP connections, and forwards them to a healthy backend. It doesn't inspect HTTP — it works at the transport layer, which keeps it simple and fast but also means it can't do path-based routing or header manipulation.

**Current feature set:**

- Round-robin load balancing using atomic counters
- Health checks (background goroutine, TCP dial every second)
- Connection retries across backends on failure
- Per-client token bucket rate limiting
- Configurable timeouts on backend connections

---

## Architecture

```text
Client
  │
  ▼
LoadBalancer (:8080)
  │
  ├── RateLimiter         ← per-client IP, token bucket
  │
  └── ServicePool
        ├── Service (localhost:9001)
        ├── Service (localhost:9002)
        └── Service (localhost:9003)
```

When a client connects:

1. The client IP is checked against the rate limiter — connection is dropped if the bucket is empty
2. The pool picks the next healthy backend via round-robin
3. A TCP connection is established to the backend (200ms timeout)
4. If it fails, the load balancer retries with the next backend
5. On success, traffic is proxied bidirectionally using `io.Copy` until either side closes

---

## Real-World Use Cases

L4 load balancers are the right tool when we don't need to inspect application-layer content — or when the protocol isn't HTTP at all.

**Database clusters** — Distributing connections to MySQL, PostgreSQL, or Redis replicas. The load balancer has no business parsing SQL; it just routes TCP connections to the right node.

**Game servers** — Low-latency multiplayer games use raw TCP or UDP. L4 keeps overhead minimal and doesn't add the per-request processing cost of HTTP parsing.

**IoT and MQTT brokers** — Devices connecting over MQTT (a publish-subscribe protocol over TCP) behind a fleet of brokers. L4 handles connection distribution without needing to understand the protocol.

**Internal infrastructure** — Services communicating over custom binary protocols or gRPC within a data center. An L4 proxy can sit in front without any knowledge of the wire format.

**High-throughput scenarios** — When connection volume is large enough that the overhead of L7 inspection (TLS termination, header parsing, routing logic) is measurable, L4 is the better option.

---

## Design Decisions & Trade-offs

### Health Checks in a Background Goroutine

Health checks run on a separate goroutine, dialing each backend every second.
If a backend fails 3 consecutive checks, it's marked unhealthy and skipped during routing. On the next successful check, it's revived.
The threshold means a brief outage won't immediately remove a backend from rotation.

### Token Bucket Rate Limiting

Each unique client IP gets its own token bucket. The capacity, refill rate, and bucket TTL are configurable — current defaults are 5 tokens, 1/second, 10s TTL. The bucket is lazy-initialized on first connection and a cleanup goroutine removes idle ones on a configurable interval (default 5s).

Token bucket (vs leaky bucket or fixed window) was chosen because it handles bursts naturally: a client can use their full capacity immediately, then gets throttled at the refill rate. Fixed window counters reset in a way that allows double-bursting at window boundaries. Sliding window logs are more accurate but expensive.

### Retry on Failure

If a backend connection fails, `handleConn` loops and tries the next backend in the pool. This is a simple but important resilience primitive — a single backend going down doesn't immediately cause client-visible errors.

### Round Robin with an Atomic Counter

The pool uses `atomic.Uint64` to track which backend is next, rather than a mutex. This means the counter is lock-free — useful if we're handling thousands of concurrent connections but works only for a fixed pool size since the counter wraps around via modulo. So backends cannot be added or removed at runtime with current design.

A mutex would be simpler to reason about but adds contention. The atomic approach pushes complexity into correctness guarantees the hardware already provides.

---

## Configuration

All values are set in `main.go` and `healthCheck`. Current defaults:

| Parameter | Default | Notes |
| --- | --- | --- |
| Listen address | `:8080` | |
| Backend addresses | `localhost:9001–9003` | |
| Health check interval | `1s` | |
| Health check dial timeout | `500ms` | |
| Failure threshold | `3` | Consecutive failures before marking unhealthy |
| Backend connection timeout | `200ms` | Per retry attempt |
| Rate limit capacity | `5` | Max burst tokens per client IP |
| Rate limit refill rate | `1/s` | |
| Rate limit bucket TTL | `10s` | How long to keep idle buckets |
| Bucket cleanup interval | `5s` | |

---

## Running It

You'll need two terminals.

**Terminal 1 — Start the backends:**

```bash
go run ./backends/main.go
```

This starts all three mock TCP servers (`localhost:9001`, `9002`, `9003`) concurrently in a single process. Each backend sends a greeting identifying itself (`hello from localhost:900X`) before echoing traffic back, so you can see which backend handled each connection.

**Terminal 2 — Start the load balancer:**

```bash
go run main.go
```

The load balancer listens on `:8080`.

**Send traffic:**

```bash
nc localhost 8080
```

Open multiple `nc` sessions to see round-robin in action. Kill one of the backend listeners to observe health checks pulling it from rotation and retries routing around it.

---

## What's Next

These are the known gaps so far:

**Idle timeouts / read-write deadlines**  
Right now connections live indefinitely once established holding one goroutine and network FD. The fix is setting `SetDeadline` on connections, then deciding how to handle the `i/o timeout` error gracefully. Certain applications like chat apps might require polling as part of application logic to keep connection alive.

**Graceful shutdown**  
Sending a SIGINT right now kills all in-flight connections immediately. A proper shutdown looks like: stops accepting new connections, waits for active ones to finish (with a timeout), then exits. We can have a `sync.WaitGroup` over active connections and a context passed down to the accept loop.

**Backpressure / connection limits**  
Without a cap on active connections, a traffic spike creates unbounded goroutines. A semaphore (buffered channel) on `handleConn` entry can be added.

**Connection pooling to backends**  
Every client connection dials a fresh TCP connection to the backend which is wasteful for short-lived connections. A pool of persistent connections per backend will reduce dial overhead.
The complication: multiplexing connections when we're proxying raw TCP requires framing the protocol — sharing a TCP stream between two clients is not possible.

**Weighted round-robin**  
The current pool treats all backends equally. In practice backends may have different capacity. Weighted round-robin will add a `weight` field to `Service` and distributes proportionally.

**Metrics**  
A simple metrics endpoint showing active connections per backend, rate-limit drops, health check states, retry rates would help when load-testing the server.

**Config Reload**
Currently, we require a restart to change backend configuration. A possibility to reload backend configuration on the fly would be nice. This also needs redesign of next backend atomic counter as mentioned above.

**Environment Variables**
The parameters configuration can be handled with a `.env` file instead of directly changing in the main code.
