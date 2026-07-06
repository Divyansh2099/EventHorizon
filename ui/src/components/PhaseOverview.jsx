import React from 'react';
import { Cpu, Server, FileCode2 } from 'lucide-react';
import MetricPanel from './MetricPanel';
import { useSpringNumber, formatBytes } from '../lib/utils';

export default function PhaseOverview({ rawMetrics, iops, rps }) {
    return (
        <div className="grid">
            <MetricPanel 
                type="kernel"
                title="OS KERNEL I/O"
                icon={Cpu}
                colorRGB="255, 42, 133"
                chartData={iops}
                metrics={[
                    { label: 'AcceptEx Calls', value: useSpringNumber(rawMetrics.accepts) },
                    { label: 'WSARecv Calls', value: useSpringNumber(rawMetrics.reads) },
                    { label: 'WSASend Calls', value: useSpringNumber(rawMetrics.writes) },
                    { label: 'Bytes In', value: useSpringNumber(rawMetrics.bytesIn, formatBytes), color: 'var(--accent)' },
                    { label: 'Bytes Out', value: useSpringNumber(rawMetrics.bytesOut, formatBytes), color: 'var(--accent)' }
                ]}
            />
            
            <MetricPanel 
                type="memory"
                title="MEMORY POOL"
                icon={Server}
                colorRGB="0, 212, 255"
                chartData={rawMetrics.connsActive}
                metrics={[
                    { label: 'Active Connections', value: useSpringNumber(rawMetrics.connsActive) },
                    { label: 'Active Buffers (4KB)', value: useSpringNumber(rawMetrics.connsActive * 2) },
                    { label: 'Total Pre-alloc Memory', value: useSpringNumber(rawMetrics.connsActive * 2 * 4096, formatBytes), color: 'var(--accent2)' },
                    { label: 'GC Pressure', value: '0.00%' }
                ]}
            />

            <MetricPanel 
                type="parser"
                title="HTTP PARSER"
                icon={FileCode2}
                colorRGB="0, 255, 157"
                chartData={rps}
                metrics={[
                    { label: 'Requests Parsed', value: useSpringNumber(rawMetrics.requestsParsed) },
                    { label: 'Parser Errors', value: useSpringNumber(rawMetrics.parserErrors) },
                    { label: 'Throughput (Req/s)', value: `${useSpringNumber(rps)} / s`, color: 'var(--accent3)' }
                ]}
            />
        </div>
    );
}
