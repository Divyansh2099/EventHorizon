import React from 'react';
import { useSpringNumber } from '../lib/utils';
import { Network, Zap } from 'lucide-react';

export default function Phase4_AcceptEx({ rawMetrics }) {
    return (
        <div className="phase-view phase4">
            <header className="phase-header">
                <h2>Phase 4: AcceptEx Connection Lifecycle</h2>
                <p>Pre-posting multiple asynchronous connection tokens to intercept spikes.</p>
            </header>
            
            <div className="metrics-row">
                <div className="stat-card">
                    <Network size={24} color="var(--accent)" />
                    <div className="stat-info">
                        <h3>Total Accepts</h3>
                        <p>{useSpringNumber(rawMetrics.accepts)}</p>
                    </div>
                </div>
                <div className="stat-card">
                    <Zap size={24} color="var(--accent2)" />
                    <div className="stat-info">
                        <h3>Pre-posted Queue</h3>
                        <p>1000 Slots Ready</p>
                    </div>
                </div>
            </div>
            
            <div className="acceptex-visualizer">
                <div className="accept-queue">
                    {Array.from({ length: 50 }).map((_, i) => (
                        <div key={i} className={`accept-slot ${i < (rawMetrics.accepts % 50) ? 'filled' : 'ready'}`}></div>
                    ))}
                </div>
                <p className="legend">Showing active pre-posted AcceptEx tokens...</p>
            </div>
        </div>
    );
}
