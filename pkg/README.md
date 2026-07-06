# pkg

**Responsibility:**
This directory contains code that is safe to be imported by external applications.

Unlike `internal/`, packages here form the public API surface of EventHorizon. This includes configuration structures, public metrics interfaces, and robust extension hooks (e.g., custom middleware definitions).
