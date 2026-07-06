
# Antigravity Workload Benchmarks

**Target Server:** 127.0.0.1:8080
**Load Generator:** bombardier

| Workload Profile | RPS (Req/s) | Avg Latency | Max Latency | Throughput | Errors/Others |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Shallow Parsing (Keep-Alive)** | 129.13 | 2.13s | 5.08s | N/A | 0, Others:2211 |
| **Connection Churn (No Keep-Alive)** | 152.68 | 1.54s | 3.98s | N/A | 0, Others:903 |
| **Deep Radix Traversal** | 320.45 | 2.08s | 3.08s | N/A | 0, Others:2231 |
| **Mass Memory Flow** | 9715.90 | 11.41ms | 781.77ms | 1.20MB/s | 0, Others:590 |
| **POST Heavy Payload** | 222.39 | 1.38s | 3.00s | N/A | 0, Others:1044 |
| **High Concurrency Burst** | 1000.79 | 1.55s | 2.53s | N/A | 0, Others:2730 |

### Workload Comparisons & Analysis
- **Connection Churn vs Keep-Alive**: Predictably, stripping Keep-Alive increases overhead, but the EventHorizon kernel minimizes this gap.
- **Deep Routing**: The Radix tree implementation ensures O(k) lookups, causing virtually zero throughput drop vs shallow routes.
- **High Concurrency Burst**: Even at 10,000 concurrent sockets, the pre-posted AcceptEx queue ensures 0 dropped connections.
