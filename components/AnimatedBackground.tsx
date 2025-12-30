
import React, { useEffect, useRef, useState } from 'react';

export const AnimatedBackground: React.FC = () => {
  const [mousePos, setMousePos] = useState({ x: 0, y: 0 });
  const [targetPos, setTargetPos] = useState({ x: 0, y: 0 });
  const requestRef = useRef<number>(0);

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      setTargetPos({ x: e.clientX, y: e.clientY });
    };
    window.addEventListener('mousemove', handleMouseMove);

    const animate = () => {
      setMousePos((prev) => ({
        x: prev.x + (targetPos.x - prev.x) * 0.08,
        y: prev.y + (targetPos.y - prev.y) * 0.08,
      }));
      requestRef.current = requestAnimationFrame(animate);
    };
    requestRef.current = requestAnimationFrame(animate);

    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      cancelAnimationFrame(requestRef.current);
    };
  }, [targetPos]);

  return (
    <div className="fixed inset-0 -z-10 overflow-hidden pointer-events-none bg-[#050505]">
      {/* 1. Deep Void Phase Grid */}
      <div 
        className="absolute inset-0 opacity-40" 
        style={{ 
          backgroundImage: `linear-gradient(#ffffff04 1px, transparent 1px), linear-gradient(90deg, #ffffff04 1px, transparent 1px)`,
          backgroundSize: '40px 40px',
          maskImage: `radial-gradient(circle 400px at ${mousePos.x}px ${mousePos.y}px, black 0%, transparent 100%)`,
          WebkitMaskImage: `radial-gradient(circle 400px at ${mousePos.x}px ${mousePos.y}px, black 0%, transparent 100%)`,
        }}
      />

      {/* 2. Neural Focus Point (Reactive Glow) */}
      <div 
        className="absolute w-[500px] h-[500px] rounded-full blur-[100px] opacity-[0.12]"
        style={{
          background: 'radial-gradient(circle, #FF9F0A 0%, #2E5BFF 50%, transparent 100%)',
          left: mousePos.x - 250,
          top: mousePos.y - 250,
        }}
      />

      {/* 3. Floating Data Bits */}
      <div className="absolute inset-0 overflow-hidden">
        {[...Array(15)].map((_, i) => (
          <div 
            key={i}
            className="absolute w-px h-8 bg-gradient-to-b from-transparent via-[#2E5BFF44] to-transparent"
            style={{
              left: `${(i * 13.7) % 100}%`,
              top: `${(i * 19.3) % 100}%`,
              transform: `translateY(${(mousePos.y - window.innerHeight/2) * (0.02 + i * 0.001)}px) translateX(${(mousePos.x - window.innerWidth/2) * (0.01)}px)`,
            }}
          />
        ))}
      </div>

      {/* 4. Scanning Horizon */}
      <div className="absolute bottom-0 left-0 w-full h-[30vh] opacity-[0.05]"
           style={{ background: 'linear-gradient(to top, #FF9F0A 0%, transparent 100%)' }} />
    </div>
  );
};
