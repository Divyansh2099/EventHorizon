# Replicating EventHorizon Benchmarks

We take performance claims very seriously. EventHorizon was designed to bypass the Go garbage collector using a ~500MB static kernel memory plane on Windows. 

To prove that the server hits **80,000+ RPS** and allocates **0 bytes on the heap**, you can replicate all benchmarks on your own Windows machine!

---

## 1. The Micro-Benchmarks (Proving Zero Allocations)
These benchmarks isolate the internal engine components (the Buffer Pool, the Radix Router, and the Zero-Copy HTTP Parser) to prove they don't trigger the Go garbage collector.

**Requirements:** Go installed.

1. Open your terminal in the repository root.
2. Run the Go benchmark suite with the memory flag enabled:
   ```bash
   go test -bench . -benchmem ./benchmarks
   ```
3. **What to look for:** Look at the far right column of the output. The `BenchmarkZeroCopyParser`, `BenchmarkBufferPool`, and `BenchmarkRouterLookup` will all output `0 B/op` and `0 allocs/op`. 

---

## 2. The End-to-End Showdown (EventHorizon vs Go Standard Library)
This benchmark tests raw HTTP throughput (Requests Per Second) under heavy concurrency, proving EventHorizon outperforms the highly optimized Go `net/http` server.

**Requirements:** Go installed, and the [Bombardier load testing tool](https://github.com/codesenberg/bombardier/releases) (download the Windows `.exe` version).

### Step 1: Start the Go Standard Library Server
In a new terminal window, start the baseline server:
```bash
go run ./cmd/stdhttp
```
*(This server will listen on `https://127.0.0.1:8083`)*

### Step 2: Start the EventHorizon Server
In a second terminal window, start the EventHorizon server:
```bash
go run ./cmd/example
```
*(This server will listen on `https://127.0.0.1:8082`)*

### Step 3: Blast Both Servers with Traffic
Open a third terminal window (where your `bombardier.exe` is located). We will hit both servers with **500 concurrent connections for 30 seconds**. *(Note: the `-k` flag tells Bombardier to ignore our self-signed TLS certificates).*

**Run against the Standard Library:**
```powershell
.\bombardier-windows-amd64.exe -c 500 -d 30s -k https://127.0.0.1:8083/api/status
```
*Note the Average RPS and Latency.*

**Run against EventHorizon:**
```powershell
.\bombardier-windows-amd64.exe -c 500 -d 30s -k https://127.0.0.1:8082/api/status
```

### 4. Analyze the Results
When running these side-by-side:
- **EventHorizon** should yield significantly higher RPS and lower latency.
- If you open **Windows Task Manager**, you will notice the `stdhttp.exe` memory usage fluctuating constantly as the Go Garbage collector desperately tries to clean up the millions of dynamic objects it creates. Meanwhile, `example.exe` (EventHorizon) will immediately claim ~500MB on boot and its memory line will remain completely flat, proving the zero-allocation kernel plane is working!
