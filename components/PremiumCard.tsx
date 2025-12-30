
import React from 'react';
import { ProximityWrapper } from './ProximityWrapper';

interface PremiumCardProps {
  children: React.ReactNode;
  className?: string;
  isAI?: boolean;
}

export const PremiumCard: React.FC<PremiumCardProps> = ({ children, className = "", isAI = false }) => {
  return (
    <ProximityWrapper maxDistance={250}>
      {(proximity) => (
        <div 
          className={`
            relative bg-[#0d0d0d] border-2 border-white/[0.1] p-10 rounded-[2.5rem] 
            transition-all duration-700 ease-out cursor-default
            hover:border-white/[0.3] hover:bg-[#121212] hover:shadow-[0_40px_80px_rgba(0,0,0,0.7)]
            ${isAI ? 'ring-4 ring-[#FF9F0A]/30' : ''}
            ${className}
          `}
          style={{
            borderColor: proximity > 0.1 ? `rgba(255, 159, 10, ${0.15 + proximity * 0.6})` : undefined,
          }}
        >
          {/* HUD Brackets (Thicker & Larger) */}
          <div className="absolute top-4 left-4 w-6 h-6 border-t-2 border-l-2 border-white/20 group-hover:border-[#FF9F0A]/60 transition-all duration-700" />
          <div className="absolute top-4 right-4 w-6 h-6 border-t-2 border-r-2 border-white/20 group-hover:border-[#FF9F0A]/60 transition-all duration-700" />
          <div className="absolute bottom-4 left-4 w-6 h-6 border-b-2 border-l-2 border-white/20 group-hover:border-[#FF9F0A]/60 transition-all duration-700" />
          <div className="absolute bottom-4 right-4 w-6 h-6 border-b-2 border-r-2 border-white/20 group-hover:border-[#FF9F0A]/60 transition-all duration-700" />

          {isAI && (
            <div className="absolute -top-4 left-10 px-6 py-2 bg-[#FF9F0A] text-black font-mono text-[12px] font-black tracking-[0.3em] uppercase rounded-xl shadow-2xl">
              Priority Objective
            </div>
          )}
          {children}
        </div>
      )}
    </ProximityWrapper>
  );
};
