<div align="center">
  <img src="./cmd/showcase/public/icons.svg" width="120" height="120" alt="EventHorizon Logo"/>
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

Read the full instructions on [How to Reproduce the Benchmarks](./getting-started/BENCHMARKS.md) on your own machine.

## 🧠 Core Architecture

Traditional Go web servers dynamically allocate buffers and connection state objects per request, which triggers the Garbage Collector. Under intense load (e.g., 100,000+ requests per second), the GC becomes a catastrophic bottleneck.

EventHorizon solves this using three custom-built engines:
1. **The RIO Ring Buffer:** Pre-allocates a massive contiguous block of physical RAM directly mapped to the Network Card driver. 
2. **The Zero-Copy Parser:** A custom HTTP/1.1 and WebSocket parsing state machine that reads data in-place out of the RIO ring without copying bytes to the Go heap.
3. **The SSPI Engine:** Instead of pulling in massive cryptographic dependencies, EventHorizon multi-threads `AcceptSecurityContext` across the Windows I/O Completion Port (IOCP) thread pool for hardware-accelerated TLS 1.3.

## 🛠 Getting Started

Due to the deep integration with the Windows Kernel, EventHorizon **requires Windows 10/11 or Windows Server** and will not compile on Linux or macOS.

For step-by-step instructions on cloning the repo, setting up the required development certificates, and running the live portfolio dashboard, please read our [Quickstart Guide](./getting-started/RUN_LOCALLY.md).

---

<div align="center">
  <i>Built for the relentless pursuit of speed.</i>
</div>
