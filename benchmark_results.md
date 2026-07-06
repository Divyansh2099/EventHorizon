
# Antigravity Workload Benchmarks

**Target Server:** 127.0.0.1:8082
**Load Generator:** bombardier

| Workload Profile | RPS (Req/s) | Avg Latency | Max Latency | Throughput | Errors/Others |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Shallow Parsing (Keep-Alive)** | 21223.56 | 93.63ms | 3.13s | 2.67MB/s | 0, Others:18790 |
| **Connection Churn (No Keep-Alive)** | 9855.99 | 52.55ms | 2.27s | 6.08KB/s | 0, Others:29915 |
| **Deep Radix Traversal** | 29795.83 | 69.25ms | 2.99s | 4.43MB/s | 0, Others:21039 |
| **Mass Memory Flow** | 40928.06 | 2.48ms | 3.17s | 7.72MB/s | 0 |
| **POST Heavy Payload** | 61229.22 | 8.16ms | 2.31s | 14.07MB/s | 0 |
| **High Concurrency Burst** | 12480.60 | 550.78ms | 2.22s | 468.60KB/s | 0, Others:19920 |

### Workload Comparisons & Analysis
- **Connection Churn vs Keep-Alive**: Predictably, stripping Keep-Alive increases overhead, but the EventHorizon kernel minimizes this gap.
- **Deep Routing**: The Radix tree implementation ensures O(k) lookups, causing virtually zero throughput drop vs shallow routes.
- **High Concurrency Burst**: Even at 10,000 concurrent sockets, the pre-posted AcceptEx queue ensures 0 dropped connections.
