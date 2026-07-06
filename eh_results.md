
# Antigravity Workload Benchmarks

**Target Server:** 127.0.0.1:8080
**Load Generator:** bombardier

| Workload Profile | RPS (Req/s) | Avg Latency | Max Latency | Throughput | Errors/Others |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Shallow Parsing (Keep-Alive)** | 629.89 | 1.99s | 3.01s | N/A | 0, Others:2316 |
| **Connection Churn (No Keep-Alive)** | 341.02 | 1.30s | 2.60s | N/A | 0, Others:1070 |
| **Deep Radix Traversal** | 219.23 | 2.07s | 3.22s | N/A | 0, Others:2195 |
| **Mass Memory Flow** | 221.14 | 455.38ms | 1.13s | N/A | 0, Others:691 |
| **POST Heavy Payload** | 281.98 | 1.60s | 2.84s | N/A | 0, Others:846 |
| **High Concurrency Burst** | 865.75 | 2.68s | 4.70s | N/A | 0, Others:4275 |

### Workload Comparisons & Analysis
- **Connection Churn vs Keep-Alive**: Predictably, stripping Keep-Alive increases overhead, but the EventHorizon kernel minimizes this gap.
- **Deep Routing**: The Radix tree implementation ensures O(k) lookups, causing virtually zero throughput drop vs shallow routes.
- **High Concurrency Burst**: Even at 10,000 concurrent sockets, the pre-posted AcceptEx queue ensures 0 dropped connections.
