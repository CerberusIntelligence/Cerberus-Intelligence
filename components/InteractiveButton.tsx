
import React from 'react';

interface InteractiveButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost';
  isLoading?: boolean;
}

export const InteractiveButton: React.FC<InteractiveButtonProps> = ({ 
  children, 
  variant = 'primary', 
  isLoading, 
  className = "", 
  ...props 
}) => {
  const baseStyles = "px-12 py-5 rounded-2xl font-black transition-all duration-500 active:scale-95 disabled:opacity-50 flex items-center justify-center gap-4 uppercase tracking-[0.3em] text-[12px]";
  
  const variants = {
    primary: "bg-[#FF9F0A] text-black hover:bg-white hover:shadow-[0_0_40px_rgba(255,159,10,0.5)] border-none",
    secondary: "bg-white text-black hover:bg-zinc-200 hover:shadow-[0_0_30px_rgba(255,255,255,0.2)] border-none",
    ghost: "bg-transparent border-2 border-white/20 text-white hover:border-[#FF9F0A] hover:text-[#FF9F0A] hover:shadow-[0_0_30px_rgba(255,159,10,0.2)]"
  };

  return (
    <button 
      className={`${baseStyles} ${variants[variant]} ${className}`}
      disabled={isLoading || props.disabled}
      {...props}
    >
      {isLoading ? (
        <div className="w-6 h-6 border-3 border-current border-t-transparent rounded-full animate-spin" />
      ) : children}
    </button>
  );
};
