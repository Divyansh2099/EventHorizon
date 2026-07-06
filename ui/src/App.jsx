import React, { useState, useEffect } from 'react';
import Sidebar from './components/Sidebar';
import PhaseOverview from './components/PhaseOverview';
import Phase1_MemoryPool from './components/Phase1_MemoryPool';
import Phase2_3_Kernel from './components/Phase2_3_Kernel';
import Phase4_AcceptEx from './components/Phase4_AcceptEx';
import Phase5_6_Router from './components/Phase5_6_Router';
import './App.css';

export default function App() {
    const [status, setStatus] = useState('CONNECTING...');
    const [activePhase, setActivePhase] = useState('overview');
    
    const [rawMetrics, setRawMetrics] = useState({
        accepts: 0, reads: 0, writes: 0, bytesIn: 0, bytesOut: 0,
        connsActive: 0, requestsParsed: 0, parserErrors: 0
    });
    
    const [lastReqs, setLastReqs] = useState(0);
    const [lastIO, setLastIO] = useState(0);

    const [iops, setIops] = useState(0);
    const [rps, setRps] = useState(0);

    useEffect(() => {
        const connect = () => {
            const evtSource = new EventSource("http://localhost:8081/stream");

            evtSource.onopen = () => {
                setStatus("STREAMING LIVE");
            };

            evtSource.onmessage = (event) => {
                try {
                    const metrics = JSON.parse(event.data);
                    
                    setRawMetrics(prev => {
                        const currentIO = metrics.reads + metrics.writes;
                        setIops(currentIO - lastIO);
                        setLastIO(currentIO);

                        const currentRps = (metrics.requestsParsed - lastReqs) * 10;
                        setRps(currentRps);
                        setLastReqs(metrics.requestsParsed);
                        
                        return metrics;
                    });
                } catch (e) {
                    console.error("Failed to process stream message", e);
                }
            };

            evtSource.onerror = () => {
                setStatus("CONNECTION LOST - RETRYING");
                evtSource.close();
                setTimeout(connect, 2000);
            };
        };
        
        connect();
    }, [lastIO, lastReqs]);

    const statusStyle = {
        background: status === 'STREAMING LIVE' ? 'rgba(0, 255, 157, 0.1)' : 'rgba(255, 42, 133, 0.1)',
        borderColor: status === 'STREAMING LIVE' ? 'rgba(0, 255, 157, 0.3)' : 'rgba(255, 42, 133, 0.3)',
        color: status === 'STREAMING LIVE' ? 'var(--accent3)' : 'var(--accent)',
    };

    const renderActivePhase = () => {
        switch(activePhase) {
            case 'phase1': return <Phase1_MemoryPool rawMetrics={rawMetrics} />;
            case 'phase23': return <Phase2_3_Kernel rawMetrics={rawMetrics} iops={iops} />;
            case 'phase4': return <Phase4_AcceptEx rawMetrics={rawMetrics} />;
            case 'phase56': return <Phase5_6_Router rawMetrics={rawMetrics} rps={rps} />;
            default: return <PhaseOverview rawMetrics={rawMetrics} iops={iops} rps={rps} />;
        }
    }

    return (
        <div className="app-container">
            <div className="page-bg" aria-hidden="true">
                <div className="hero-bg"></div>
                <div className="hero-glow"></div>
            </div>
            
            <Sidebar activePhase={activePhase} setActivePhase={setActivePhase} />
            
            <main className="main-content">
                <header>
                    <div>
                        <h1>EventHorizon</h1>
                        <div className="subtitle">ULTRA-LOW LATENCY TELEMETRY // 100MS STREAM</div>
                    </div>
                    <div className="status-container" style={statusStyle}>
                        <div className="status-dot"></div>
                        <span>{status}</span>
                    </div>
                </header>

                <div className="phase-container">
                    {renderActivePhase()}
                </div>
            </main>
        </div>
    );
}
