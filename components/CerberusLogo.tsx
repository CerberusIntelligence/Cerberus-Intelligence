
import React from 'react';

export const CerberusLogo: React.FC<{ size?: number; className?: string }> = ({ size = 32, className = "" }) => (
  <svg 
    width={size} 
    height={size} 
    viewBox="0 0 64 64" 
    fill="none" 
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    {/* Digital Foundation */}
    <rect x="22" y="44" width="20" height="2" fill="currentColor" opacity="0.2" />
    <rect x="26" y="48" width="12" height="1" fill="currentColor" opacity="0.1" />

    {/* Central Neural Head */}
    <path 
      d="M32 6L25 15L23 21L27 28L32 38L37 28L41 21L39 15L32 6Z" 
      fill="currentColor" 
      className="opacity-90"
    />
    {/* Data Eyes */}
    <rect x="28" y="16" width="3" height="1" fill="zinc-950" opacity="0.8" />
    <rect x="33" y="16" width="3" height="1" fill="zinc-950" opacity="0.8" />
    
    {/* Left Signal Head */}
    <path 
      d="M20 28C16 28 8 24 5 32C3.5 36 6 42 12 44L18 40" 
      stroke="currentColor" 
      strokeWidth="2.5" 
      strokeLinecap="square" 
    />
    <rect x="5" y="32" width="2" height="2" fill="currentColor" />

    {/* Right Signal Head */}
    <path 
      d="M44 28C48 28 56 24 59 32C60.5 36 58 42 52 44L46 40" 
      stroke="currentColor" 
      strokeWidth="2.5" 
      strokeLinecap="square" 
    />
    <rect x="57" y="32" width="2" height="2" fill="currentColor" />

    {/* Signal Links */}
    <path 
      d="M26 34C26 34 28 44 32 44C36 44 38 34 38 34" 
      stroke="currentColor" 
      strokeWidth="1.5" 
      opacity="0.3"
    />
  </svg>
);
