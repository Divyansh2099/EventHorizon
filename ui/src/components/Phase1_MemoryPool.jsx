import React from 'react';
import { useSpringNumber, formatBytes } from '../lib/utils';
import { Server, Database } from 'lucide-react';

export default function Phase1_MemoryPool({ rawMetrics }) {
    // Determine the number of active blocks based on connections
    const activeBlocks = rawMetrics.connsActive;
    
    // Create a visual grid of memory blocks (e.g., 64 blocks max in the UI for performance)
    const renderBlocks = () => {
        const totalBlocks = 64; 
        const blocks = [];
        for(let i = 0; i < totalBlocks; i++) {
            const isActive = i < activeBlocks;
            blocks.push(
                <div key={i} className={`memory-block ${isActive ? 'active' : ''}`}>
                    <span className="hex">0x{Math.floor(Math.random()*16777215).toString(16).padStart(6, '0')}</span>
                    <span className="status">{isActive ? 'IN USE' : 'FREE'}</span>
                </div>
            );
        }
        return blocks;
    };

    return (
        <div className="phase-view phase1">
            <header className="phase-header">
                <h2>Phase 1: Zero-Allocation Pool Manager</h2>
                <p>Strict allocation-free structural recycling pools.</p>
            </header>
            
            <div className="metrics-row">
                <div className="stat-card">
                    <Database size={24} color="var(--accent2)" />
                    <div className="stat-info">
                        <h3>Buffer Pool</h3>
                        <p>{useSpringNumber(rawMetrics.connsActive * 4096, formatBytes)} Allocated</p>
                    </div>
                </div>
                <div className="stat-card">
                    <Server size={24} color="var(--accent)" />
                    <div className="stat-info">
                        <h3>Context Pool</h3>
                        <p>{useSpringNumber(rawMetrics.connsActive)} Active OverlappedCtx</p>
                    </div>
                </div>
            </div>

            <div className="memory-grid-container">
                <h3>[4096]byte Array Slices</h3>
                <div className="memory-grid">
                    {renderBlocks()}
                </div>
            </div>
        </div>
    );
}
