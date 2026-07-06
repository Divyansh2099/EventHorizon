import React from 'react';
import { Layers, Zap, Network, Brackets, Globe, LayoutDashboard } from 'lucide-react';

export default function Sidebar({ activePhase, setActivePhase }) {
    const navItems = [
        { id: 'overview', label: 'Overview', icon: LayoutDashboard },
        { id: 'phase1', label: 'Phase 1: Memory Pool', icon: Layers },
        { id: 'phase23', label: 'Phase 2 & 3: Kernel', icon: Zap },
        { id: 'phase4', label: 'Phase 4: AcceptEx', icon: Network },
        { id: 'phase56', label: 'Phase 5 & 6: Router', icon: Brackets },
    ];

    return (
        <aside className="w-[260px] bg-[#020202]/95 border-r border-line backdrop-blur-2xl flex flex-col py-6 z-[100] shadow-[4px_0_24px_rgba(0,0,0,0.5)]">
            <div className="px-6 pb-6 border-b border-line mb-5">
                <h2 className="text-2xl font-extrabold text-gradient m-0">EventHorizon</h2>
            </div>
            <nav className="flex flex-col gap-2 px-4">
                {navItems.map(item => {
                    const Icon = item.icon;
                    return (
                        <button 
                            key={item.id}
                            className={`nav-btn ${activePhase === item.id ? 'active' : ''}`}
                            onClick={() => setActivePhase(item.id)}
                        >
                            <Icon size={18} />
                            <span>{item.label}</span>
                        </button>
                    );
                })}
            </nav>
        </aside>
    );
}
