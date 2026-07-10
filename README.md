<div align="center">
  <h1>EventHorizon Framework</h1>
  <p><strong>A Zero-Allocation, Hardware-Accelerated Web Framework for Go on Windows.</strong></p>
</div>

<br>

**EventHorizon** is a high-performance HTTP framework designed to push the physical limits of modern network interface cards by completely bypassing the Go Garbage Collector (GC). 

By leveraging native **Windows Registered I/O (RIO)** and **SSPI (Schannel)**, EventHorizon pre-allocates an immovable 500MB physical kernel plane on boot. During heavy traffic, the server achieves blistering speeds with **0 heap allocations per request**.

## 🚀 Performance Showcase

In a heavy 30-second concurrency load test (500 connections) against the official, highly-optimized Go `net/http` standard library, EventHorizon demonstrated substantial throughput advantages:

| Performance Metric | ⚡ EventHorizon | 🐢 Go `net/http` | Difference |
| :--- | :--- | :--- | :--- |
| **Average RPS** | **84,859.55** req/sec | 58,539.26 req/sec | **~45% Faster** |
| **Peak RPS (Burst)** | **119,841.87** req/sec | 90,811.46 req/sec | **~32% Higher Burst** |
| **Average Latency** | **5.87 ms** | 8.52 ms | **~31% Lower Latency**|

### Micro-Benchmark Analysis (Standard Request)

| Benchmark Scenario | Framework | Speed (ns/op) | Throughput | Memory (B/op) | Allocs/op |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **HTTP Parser** | ⚡ **EventHorizon** | **~398 ns/op** | **~308 MB/s** | **0 B/op** | **0** |
| | 🐢 Go `net/http` | ~5,700 ns/op | ~21 MB/s | 5,266 B/op | 13 |
| **Router Lookup (Basic Path)** | ⚡ **EventHorizon** | **~60 ns/op** | - | **0 B/op** | **0** |
| | 🐢 Go `http.ServeMux` | ~135 ns/op | - | 0 B/op | 0 |
| **Router Lookup (Parametric)**| ⚡ **EventHorizon** | **~75 ns/op** | - | **0 B/op** | **0** |
| **Connection Lifecycle** | ⚡ **EventHorizon** | **~281.9 ns/op** | - | **56 B/op** | **2** |

*For more details on our zero-allocation testing methodology, read the full instructions on [How to Reproduce the Benchmarks](./getting-started/BENCHMARKS.md) or see our [Benchmarks Readme](./benchmarks/README.md).*

## 🧠 Core Architecture & Features

Traditional Go web servers dynamically allocate buffers and connection state objects per request, which triggers the Garbage Collector. Under intense load (e.g., 100,000+ requests per second), the GC becomes a catastrophic bottleneck.

EventHorizon solves this using several custom-built engines:

| Component | Description | Benefit |
| :--- | :--- | :--- |
| **The RIO Ring Buffer** | Pre-allocates a massive contiguous block of physical RAM directly mapped to the Network Card driver using Windows Registered I/O. | Eliminates per-request buffer allocations and minimizes kernel context switching. |
| **Zero-Copy Parser** | A custom HTTP/1.1 and WebSocket parsing state machine that reads data in-place out of the RIO ring. | `0` heap allocations per parse. Up to 38x faster than standard `net/http`. |
| **SSPI Engine** | Hardware-accelerated TLS 1.3 by multi-threading `AcceptSecurityContext` across the Windows I/O Completion Port (IOCP) thread pool. | Bypasses heavy cryptographic dependencies and leverages native Windows crypto APIs. |
| **Lock-Free Radix Router** | Parses parametric routes (e.g., `/users/:id`) strictly via zero-copy byte offsets against the pinned RIO memory buffer. | Constant time lookups with zero heap overhead regardless of path depth. |
| **Static Memory Pool** | Backs dynamic byte needs with static `[4096]byte` arrays fetched from a custom pool manager. | Prevents heap escapes and guarantees allocation-free steady-state operation. |

## 📁 Project Structure

| Directory | Purpose |
| :--- | :--- |
| `cmd/` | Application entrypoints (e.g., the main server binary). |
| `pkg/` | Core packages containing the internal engines (e.g., `tls`, `parser`, `router`, `rio`). |
| `benchmarks/` | Targeted micro-benchmarks that isolate and stress test individual components. |
| `examples/` | Example code demonstrating how to build apps using the EventHorizon framework. |
| `getting-started/`| Guides on setting up the environment, certificates, and running locally. |
| `docs/` | Progress logs and detailed documentation of the architecture plan. |

## 🛠 Getting Started

Due to the deep integration with the Windows Kernel, EventHorizon **requires Windows 10/11 or Windows Server** and will not compile on Linux or macOS.

For step-by-step instructions on cloning the repo, setting up the required development certificates, and running the live portfolio dashboard, please read our [Quickstart Guide](./getting-started/RUN_LOCALLY.md).

---

<div align="center">
  <i>Built for the relentless pursuit of speed.</i>
</div>
