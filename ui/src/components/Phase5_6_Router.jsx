import React from 'react';
import { useSpringNumber } from '../lib/utils';
import { Brackets, Globe } from 'lucide-react';

export default function Phase5_6_Router({ rawMetrics, rps }) {
    return (
        <div className="phase-view phase56">
            <header className="phase-header">
                <h2>Phase 5 & 6: Zero-Copy Parser & Lock-Free Radix</h2>
                <p>Parsing text-based HTTP headers entirely through array indexing.</p>
            </header>
            
            <div className="metrics-row">
                <div className="stat-card">
                    <Brackets size={24} color="var(--accent3)" />
                    <div className="stat-info">
                        <h3>HTTP Throughput</h3>
                        <p>{useSpringNumber(rps)} Req/s</p>
                    </div>
                </div>
                <div className="stat-card">
                    <Globe size={24} color="var(--accent)" />
                    <div className="stat-info">
                        <h3>Requests Parsed</h3>
                        <p>{useSpringNumber(rawMetrics.requestsParsed)}</p>
                    </div>
                </div>
            </div>

            <div className="parser-visualizer">
                <h3>Zero-Copy Slice References</h3>
                <div className="byte-array">
                    <span className="method">GET</span> 
                    <span className="uri">/api/v1/stream</span> 
                    <span className="version">HTTP/1.1</span>
                    <span className="crlf">\r\n</span>
                    <span className="header">Host: localhost:8080</span>
                    <span className="crlf">\r\n</span>
                </div>
                <div className="pointers">
                    <div className="pointer-label">MethodStart/End</div>
                    <div className="pointer-label">URIStart/End</div>
                </div>
            </div>
        </div>
    );
}
