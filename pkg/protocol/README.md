# internal/protocol

**Responsibility:**
This package abstracts higher-level protocol specifics (like HTTP/1.1 chunked transfer encoding, and future HTTP/2 framing) away from the core zero-copy parser.

**Extension Points:**
As EventHorizon evolves, ALPN (Application-Layer Protocol Negotiation) and TLS integration will interact with this package to determine how the raw socket bytes should be interpreted.
