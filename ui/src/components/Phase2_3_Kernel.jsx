import React from 'react';
import { useSpringNumber } from '../lib/utils';
import { Cpu, Activity } from 'lucide-react';

export default function Phase2_3_Kernel({ rawMetrics, iops }) {
    return (
        <div className="phase-view phase23">
            <header className="phase-header">
                <h2>Phase 2 & 3: WinSock2 & IOCP Proactor</h2>
                <p>Continuous kernel event draining without per-connection goroutines.</p>
            </header>
            
            <div className="metrics-row">
                <div className="stat-card">
                    <Cpu size={24} color="var(--accent)" />
                    <div className="stat-info">
                        <h3>I/O Operations / Sec</h3>
                        <p>{useSpringNumber(iops)} IOPS</p>
                    </div>
                </div>
                <div className="stat-card">
                    <Activity size={24} color="var(--accent3)" />
                    <div className="stat-info">
                        <h3>Worker Threads</h3>
                        <p>16 Fixed Goroutines</p>
                    </div>
                </div>
            </div>

            <div className="iocp-visualizer">
                <h3>IOCP Kernel Queue</h3>
                <div className="queue-container">
                    <div className="worker-pool">
                        {Array.from({ length: 16 }).map((_, i) => (
                            <div key={i} className={`worker-thread ${Math.random() > 0.5 ? 'busy' : 'idle'}`}>
                                <span>W-{i}</span>
                                <div className="pulse-dot"></div>
                            </div>
                        ))}
                    </div>
                    <div className="queue-pipeline">
                        <div className="packet incoming" style={{ animationDuration: `${Math.max(0.1, 1000/Math.max(1, iops))}s` }}></div>
                        <div className="packet incoming" style={{ animationDuration: `${Math.max(0.1, 1000/Math.max(1, iops))}s`, animationDelay: '0.2s' }}></div>
                    </div>
                </div>
            </div>
        </div>
    );
}
