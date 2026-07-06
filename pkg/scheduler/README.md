# internal/scheduler

**Responsibility:**
This package provides low-latency timer management and background task scheduling, bypassing standard Go time/ticker mechanisms when they introduce too much GC pressure.

**Design:**
Uses a hierarchical timing wheel (Hashed and Hierarchical Timing Wheels) to manage thousands of concurrent connection timeouts (e.g., keep-alive limits, slowloris protection) without requiring a goroutine per connection.
