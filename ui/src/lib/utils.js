import { useState, useEffect, useRef } from 'react';

export const formatNum = (num) => new Intl.NumberFormat().format(Math.floor(num));

export const formatBytes = (bytes) => {
    if (bytes <= 0.1) return '0 B';
    const k = 1024, sizes = ['B', 'KB', 'MB', 'GB'], i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

export function useSpringNumber(target, formatter = formatNum) {
    const [current, setCurrent] = useState(target);
    const state = useRef({
        current: target,
        target: target,
        velocity: 0,
        lastTime: performance.now(),
        animating: false
    });

    useEffect(() => {
        state.current.target = target;
        if (!state.current.animating) {
            state.current.animating = true;
            state.current.lastTime = performance.now();
            
            const tick = () => {
                const now = performance.now();
                const dt = Math.min((now - state.current.lastTime) / 1000, 0.03);
                state.current.lastTime = now;
                
                const tension = 80;
                const friction = 12;
                const force = -tension * (state.current.current - state.current.target) - friction * state.current.velocity;
                
                state.current.velocity += force * dt;
                state.current.current += state.current.velocity * dt;
                
                if (Math.abs(state.current.target - state.current.current) < 0.5 && Math.abs(state.current.velocity) < 0.5) {
                    state.current.current = state.current.target;
                    setCurrent(state.current.current);
                    state.current.animating = false;
                } else {
                    setCurrent(state.current.current);
                    requestAnimationFrame(tick);
                }
            };
            requestAnimationFrame(tick);
        }
    }, [target]);

    return formatter(current);
}
