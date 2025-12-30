import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { CerberusLogo } from '../components/CerberusLogo';
import { InteractiveButton } from '../components/InteractiveButton';
import { Search, LogOut } from 'lucide-react';

const DashboardPage: React.FC = () => {
  const { user, signOut } = useAuth();
  const [searchQuery, setSearchQuery] = useState('');

  // TODO: Implement product fetching and display
  // This is a placeholder - will be enhanced with Gemini API integration

  return (
    <div className="min-h-screen bg-black text-white">
      {/* Header */}
      <nav className="border-b border-white/10 bg-black/80 backdrop-blur-xl sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
          <Link to="/" className="flex items-center gap-4">
            <div className="bg-[#FF9F0A] p-3 rounded-xl">
              <CerberusLogo size={32} />
            </div>
            <span className="font-display text-2xl font-black">DASHBOARD</span>
          </Link>

          <div className="flex items-center gap-6">
            <span className="text-sm font-mono text-zinc-400">{user?.email}</span>
            <InteractiveButton
              onClick={signOut}
              variant="ghost"
              className="px-6 py-3"
            >
              <LogOut size={18} className="mr-2" />
              Sign Out
            </InteractiveButton>
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <div className="max-w-7xl mx-auto px-6 py-12">
        {/* Search Bar */}
        <div className="mb-12">
          <h1 className="font-display text-6xl font-black mb-6">
            Product <span className="text-[#FF9F0A]">Intelligence</span>
          </h1>
          <div className="relative">
            <Search className="absolute left-6 top-1/2 -translate-y-1/2 text-zinc-500" size={24} />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search niches (e.g., Luxury Wellness Tech, Smart Home Devices)..."
              className="w-full bg-zinc-900/50 border-2 border-white/10 rounded-2xl pl-16 pr-6 py-6 text-lg text-white placeholder:text-zinc-600 focus:outline-none focus:border-[#FF9F0A] transition-colors"
            />
          </div>
        </div>

        {/* Products Grid - Placeholder */}
        <div className="text-center py-32">
          <div className="inline-block bg-zinc-900/50 border-2 border-white/10 rounded-3xl p-12">
            <h3 className="font-display text-3xl font-black text-white mb-4">
              Dashboard Under Construction
            </h3>
            <p className="text-zinc-400 font-mono mb-8">
              Product grid, detailed analytics, and enhanced Gemini integration coming soon.
            </p>
            <Link to="/">
              <InteractiveButton className="px-8 py-4">
                Back to Home
              </InteractiveButton>
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
};

export default DashboardPage;
