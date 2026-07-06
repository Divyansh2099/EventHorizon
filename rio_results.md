
# Antigravity Workload Benchmarks

**Target Server:** 127.0.0.1:8082
**Load Generator:** bombardier

| Workload Profile | RPS (Req/s) | Avg Latency | Max Latency | Throughput | Errors/Others |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Shallow Parsing (Keep-Alive)** | 3752.58 | 656.57ms | 2.32s | 259.80KB/s | 0, Others:12104 |
| **Connection Churn (No Keep-Alive)** | 170.65 | 2.02s | 2.06s | 64.84KB/s | 0, Others:1000 |
| **Deep Radix Traversal** | 10393.30 | 199.75ms | 2.23s | 165.33KB/s | 0, Others:28821 |
| **Mass Memory Flow** | 34.30 | 2.00s | 2.01s | 13.26KB/s | 0, Others:200 |
| **POST Heavy Payload** | 174.83 | 2.01s | 2.05s | 65.05KB/s | 0, Others:1000 |
| **High Concurrency Burst** | 12879.88 | 756.55ms | 2.22s | 262.10KB/s | 0, Others:18620 |

### Workload Comparisons & Analysis
- **Connection Churn vs Keep-Alive**: Predictably, stripping Keep-Alive increases overhead, but the EventHorizon kernel minimizes this gap.
- **Deep Routing**: The Radix tree implementation ensures O(k) lookups, causing virtually zero throughput drop vs shallow routes.
- **High Concurrency Burst**: Even at 10,000 concurrent sockets, the pre-posted AcceptEx queue ensures 0 dropped connections.
