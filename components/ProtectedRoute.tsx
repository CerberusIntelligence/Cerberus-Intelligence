import React from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { isSupabaseConfigured } from '../services/supabaseClient';

interface ProtectedRouteProps {
  children: React.ReactNode;
  requirePayment?: boolean;
}

export const ProtectedRoute: React.FC<ProtectedRouteProps> = ({
  children,
  requirePayment = true
}) => {
  const { user, hasActiveAccess, isLoading } = useAuth();

  // In demo mode (Supabase not configured), allow access to everything
  if (!isSupabaseConfigured) {
    console.log('Demo mode: Allowing access to protected route');
    return <>{children}</>;
  }

  // Show loading state while checking authentication
  if (isLoading) {
    return (
      <div className="min-h-screen bg-black flex items-center justify-center">
        <div className="text-center">
          <div className="w-32 h-32 border-6 border-white/5 border-t-[#FF9F0A] rounded-full animate-spin mx-auto mb-8" />
          <p className="text-white font-mono text-lg uppercase tracking-widest">
            Verifying Access...
          </p>
        </div>
      </div>
    );
  }

  // Redirect to login if not authenticated
  if (!user) {
    return <Navigate to="/login" replace />;
  }

  // Check if payment is required and user has active access
  if (requirePayment && !hasActiveAccess) {
    return <Navigate to="/login?payment_required=true" replace />;
  }

  // User is authenticated and has access (or payment not required)
  return <>{children}</>;
};
