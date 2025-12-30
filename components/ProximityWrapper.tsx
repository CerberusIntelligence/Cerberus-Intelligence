
import React, { useState, useRef, useEffect } from 'react';

interface ProximityWrapperProps {
  children: (proximity: number) => React.ReactNode;
  maxDistance?: number;
}

export const ProximityWrapper: React.FC<ProximityWrapperProps> = ({ children, maxDistance = 300 }) => {
  const [proximity, setProximity] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!containerRef.current) return;

      const rect = containerRef.current.getBoundingClientRect();
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;

      const distance = Math.sqrt(
        Math.pow(e.clientX - centerX, 2) + Math.pow(e.clientY - centerY, 2)
      );

      // 1 when at center, 0 when outside maxDistance
      const closeness = Math.max(0, 1 - distance / maxDistance);
      setProximity(closeness);
    };

    window.addEventListener('mousemove', handleMouseMove);
    return () => window.removeEventListener('mousemove', handleMouseMove);
  }, [maxDistance]);

  return (
    <div ref={containerRef} className="relative inline-block w-full">
      {children(proximity)}
    </div>
  );
};
