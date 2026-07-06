import React, { useRef, useEffect } from 'react';

export default function SparklineCanvas({ data, colorRGB }) {
    const canvasRef = useRef(null);
    const renderState = useRef({
        targetMax: 1,
        currentMax: 1,
        lastPushTime: performance.now(),
        updateInterval: 100,
        data: new Array(60).fill(0)
    });

    useEffect(() => {
        // When data changes, push the new value
        renderState.current.data.shift();
        renderState.current.data.push(data);
        if (data > renderState.current.targetMax) {
            renderState.current.targetMax = data;
        }
        renderState.current.lastPushTime = performance.now();
    }, [data]);

    useEffect(() => {
        const canvas = canvasRef.current;
        const ctx = canvas.getContext('2d');
        const dpr = window.devicePixelRatio || 1;
        const rect = canvas.getBoundingClientRect();
        canvas.width = rect.width * dpr;
        canvas.height = rect.height * dpr;
        ctx.scale(dpr, dpr);

        const width = rect.width;
        const height = rect.height;
        let animationFrameId;

        const render = (now) => {
            const state = renderState.current;
            state.targetMax *= 0.992;
            if (state.targetMax < 1) state.targetMax = 1;
            
            state.currentMax += (state.targetMax - state.currentMax) * 0.1;
            
            const elapsed = now - state.lastPushTime;
            const progress = Math.min(elapsed / state.updateInterval, 1.0);
            
            const step = width / (state.data.length - 2);
            const offsetX = (1 - progress) * step;

            ctx.clearRect(0, 0, width, height);
            
            ctx.beginPath();
            ctx.strokeStyle = 'rgba(255,255,255,0.03)';
            ctx.lineWidth = 1;
            for(let i = -offsetX; i < width; i += 30) {
                ctx.moveTo(i, 0);
                ctx.lineTo(i, height);
            }
            for(let i=1; i<4; i++) {
                const y = (height / 4) * i;
                ctx.moveTo(0, y);
                ctx.lineTo(width, y);
            }
            ctx.stroke();

            const padding = 15;
            const availableHeight = height - padding * 2;

            ctx.beginPath();
            ctx.strokeStyle = 'rgb(' + colorRGB + ')';
            ctx.lineWidth = 3;
            ctx.lineJoin = 'round';
            ctx.lineCap = 'round';
            
            ctx.shadowBlur = 18;
            ctx.shadowColor = 'rgba(' + colorRGB + ', 1)';
            
            for (let i = 0; i < state.data.length; i++) {
                const x = (i - 1) * step + (step - offsetX);
                const y = (height - padding) - (state.data[i] / state.currentMax) * availableHeight;
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            }
            ctx.stroke();
            
            ctx.shadowBlur = 0;

            const gradient = ctx.createLinearGradient(0, 0, 0, height);
            gradient.addColorStop(0, 'rgba(' + colorRGB + ', 0.4)');
            gradient.addColorStop(1, 'rgba(' + colorRGB + ', 0.0)');
            
            ctx.lineTo(width, height);
            ctx.lineTo(0, height);
            ctx.fillStyle = gradient;
            ctx.fill();

            animationFrameId = requestAnimationFrame(render);
        };

        animationFrameId = requestAnimationFrame(render);
        return () => cancelAnimationFrame(animationFrameId);
    }, [colorRGB]);

    return <canvas ref={canvasRef} style={{ width: '100%', height: '140px', marginTop: '35px', borderRadius: '12px' }} />;
}
