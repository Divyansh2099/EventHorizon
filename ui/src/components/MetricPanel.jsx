import React from 'react';
import SparklineCanvas from './SparklineCanvas';

export default function MetricPanel({ type, title, icon: Icon, metrics, chartData, colorRGB }) {
    return (
        <div className={`panel-wrapper ${type}`}>
            <div className="panel">
                <div className="panel-header">
                    <Icon size={20} />
                    {title}
                </div>
                
                {metrics.map((m, idx) => (
                    <div className="metric-row" key={idx}>
                        <span className="metric-label">{m.label}</span>
                        <span className="metric-value" style={{ color: m.color || '#fff' }}>
                            {m.value}
                        </span>
                    </div>
                ))}
                
                <SparklineCanvas data={chartData} colorRGB={colorRGB} />
            </div>
        </div>
    );
}
