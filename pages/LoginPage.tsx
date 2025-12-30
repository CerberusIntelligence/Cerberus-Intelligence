import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { CerberusLogo } from '../components/CerberusLogo';
import { InteractiveButton } from '../components/InteractiveButton';
import { AnimatedBackground } from '../components/AnimatedBackground';
import { isSupabaseConfigured } from '../services/supabaseClient';

const LoginPage: React.FC = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isSignUp, setIsSignUp] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');

  const { signIn, signUp, hasActiveAccess } = useAuth();
  const navigate = useNavigate();

  // In demo mode, show a message and allow direct access
  useEffect(() => {
    if (!isSupabaseConfigured) {
      console.log('Demo mode active - Supabase not configured');
    }
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setIsLoading(true);

    try {
      // In demo mode, just navigate to dashboard
      if (!isSupabaseConfigured) {
        console.log('Demo mode: Bypassing authentication');
        setTimeout(() => {
          navigate('/dashboard');
        }, 500);
        return;
      }

      if (isSignUp) {
        await signUp(email, password);
        // After signup, user needs to pay - show payment modal
        alert('Signup successful! Payment integration coming soon. Access granted for demo.');
        navigate('/dashboard');
      } else {
        await signIn(email, password);
        if (hasActiveAccess) {
          navigate('/dashboard');
        } else {
          alert('Payment required. Payment integration coming soon. Access granted for demo.');
          navigate('/dashboard');
        }
      }
    } catch (err: any) {
      setError(err.message || 'Authentication failed');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-black text-white relative overflow-hidden">
      <AnimatedBackground />

      <div className="relative z-10 min-h-screen flex items-center justify-center px-6">
        <div className="max-w-md w-full">
          {/* Logo Header */}
          <div className="text-center mb-12">
            <div className="inline-flex items-center gap-4 mb-6">
              <div className="bg-[#FF9F0A] p-4 rounded-2xl">
                <CerberusLogo size={48} />
              </div>
            </div>
            <h1 className="font-display text-5xl font-black text-white mb-4 tracking-tight">
              {isSignUp ? 'JOIN PROTOCOL' : 'ACCESS PROTOCOL'}
            </h1>
            <p className="text-zinc-400 font-mono text-sm uppercase tracking-widest">
              Secure Intelligence Hub
            </p>
          </div>

          {/* Demo Mode Notice */}
          {!isSupabaseConfigured && (
            <div className="bg-[#FF9F0A]/10 border-2 border-[#FF9F0A]/50 rounded-2xl p-6 mb-6 text-center">
              <p className="text-[#FF9F0A] font-mono text-sm font-black uppercase mb-2">
                ⚡ Demo Mode Active
              </p>
              <p className="text-zinc-300 font-mono text-xs">
                Supabase not configured. Click "Sign In" to access dashboard directly.
              </p>
            </div>
          )}

          {/* Login Form */}
          <div className="bg-zinc-900/40 border-2 border-white/10 rounded-3xl p-10 backdrop-blur-xl">
            <form onSubmit={handleSubmit} className="space-y-6">
              {error && (
                <div className="bg-red-500/10 border-2 border-red-500/50 rounded-xl p-4 text-red-400 text-sm font-mono">
                  {error}
                </div>
              )}

              <div>
                <label className="block text-sm font-mono uppercase tracking-widest text-zinc-400 mb-3">
                  Email
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full bg-black border-2 border-white/10 rounded-xl px-6 py-4 text-white font-mono focus:outline-none focus:border-[#FF9F0A] transition-colors"
                  placeholder="your@email.com"
                  required
                />
              </div>

              <div>
                <label className="block text-sm font-mono uppercase tracking-widest text-zinc-400 mb-3">
                  Password
                </label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full bg-black border-2 border-white/10 rounded-xl px-6 py-4 text-white font-mono focus:outline-none focus:border-[#FF9F0A] transition-colors"
                  placeholder="••••••••"
                  required
                />
              </div>

              <InteractiveButton
                type="submit"
                className="w-full py-5 text-lg rounded-xl"
                isLoading={isLoading}
              >
                {isSignUp ? 'Create Account' : 'Sign In'}
              </InteractiveButton>
            </form>

            <div className="mt-8 text-center">
              <button
                onClick={() => setIsSignUp(!isSignUp)}
                className="text-zinc-400 hover:text-[#FF9F0A] font-mono text-sm uppercase tracking-widest transition-colors"
              >
                {isSignUp ? 'Already have an account? Sign In' : 'Need an account? Sign Up'}
              </button>
            </div>
          </div>

          {isSignUp && (
            <div className="mt-8 bg-[#FF9F0A]/10 border-2 border-[#FF9F0A]/30 rounded-2xl p-6 text-center">
              <p className="text-[#FF9F0A] font-black font-mono text-lg mb-2">
                $350 - 7 DAY ACCESS
              </p>
              <p className="text-zinc-400 text-sm font-mono">
                Full product intelligence • Unlimited searches • Export capabilities
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default LoginPage;
