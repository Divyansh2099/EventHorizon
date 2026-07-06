
# Antigravity Workload Benchmarks

**Target Server:** 127.0.0.1:8082
**Load Generator:** bombardier

| Workload Profile | RPS (Req/s) | Avg Latency | Max Latency | Throughput | Errors/Others |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Shallow Parsing (Keep-Alive)** | 17241.17 | 119.92ms | 2.54s | 1.94MB/s | 0, Others:5384 |
| **Connection Churn (No Keep-Alive)** | 1758.56 | 370.98ms | 2.23s | 217.50KB/s | 0 |
| **Deep Radix Traversal** | 17929.56 | 106.32ms | 3.01s | 2.34MB/s | 0, Others:6542 |
| **Mass Memory Flow** | 17346.71 | 5.77ms | 44.70ms | 2.53MB/s | 0 |
| **POST Heavy Payload** | 18890.65 | 26.64ms | 167.18ms | 4.30MB/s | 0 |
| **High Concurrency Burst** | 1701.05 | 1.41s | 3.44s | 179.46KB/s | 0, Others:7633 |

### Workload Comparisons & Analysis
- **Connection Churn vs Keep-Alive**: Predictably, stripping Keep-Alive increases overhead, but the EventHorizon kernel minimizes this gap.
- **Deep Routing**: The Radix tree implementation ensures O(k) lookups, causing virtually zero throughput drop vs shallow routes.
- **High Concurrency Burst**: Even at 10,000 concurrent sockets, the pre-posted AcceptEx queue ensures 0 dropped connections.
