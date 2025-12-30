import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { getDetailedProductData } from '../services/geminiService';
import { DetailedProduct } from '../types';
import { CerberusLogo } from '../components/CerberusLogo';
import { InteractiveButton } from '../components/InteractiveButton';
import { ProductCard } from '../components/ProductCard';
import { AnimatedBackground } from '../components/AnimatedBackground';
import { Search, LogOut, Filter, Cpu, Clock } from 'lucide-react';
import { accessHelpers } from '../services/supabaseClient';

const DashboardPage: React.FC = () => {
  const { user, userAccess, signOut } = useAuth();
  const [searchQuery, setSearchQuery] = useState('');
  const [products, setProducts] = useState<DetailedProduct[]>([]);
  const [filteredProducts, setFilteredProducts] = useState<DetailedProduct[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [selectedFilter, setSelectedFilter] = useState<'all' | 'high-growth' | 'low-saturation'>('all');

  // Calculate days remaining
  const daysRemaining = userAccess ? accessHelpers.getDaysRemaining(userAccess) : 0;

  // Auto-search on mount with default niche
  useEffect(() => {
    handleSearch('trending products');
  }, []);

  // Filter products based on search and filter selection
  useEffect(() => {
    let filtered = products;

    // Apply search filter
    if (searchQuery.trim()) {
      filtered = filtered.filter(
        (p) =>
          p.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
          p.niche.toLowerCase().includes(searchQuery.toLowerCase())
      );
    }

    // Apply category filter
    if (selectedFilter === 'high-growth') {
      filtered = filtered.filter((p) => p.growthPotential >= 70);
    } else if (selectedFilter === 'low-saturation') {
      filtered = filtered.filter((p) => p.marketSaturation <= 40);
    }

    setFilteredProducts(filtered);
  }, [searchQuery, products, selectedFilter]);

  const handleSearch = async (query?: string) => {
    const searchTerm = query || searchQuery || 'trending products';

    setIsLoading(true);
    setError('');

    try {
      const data = await getDetailedProductData(searchTerm);
      setProducts(data);
    } catch (err: any) {
      setError(err.message || 'Failed to load products');
      setProducts([]);
    } finally {
      setIsLoading(false);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      handleSearch();
    }
  };

  return (
    <div className="min-h-screen bg-black text-white">
      <AnimatedBackground />

      {/* Header */}
      <nav className="border-b border-white/10 bg-black/80 backdrop-blur-xl sticky top-0 z-50 relative">
        <div className="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
          <Link to="/" className="flex items-center gap-4 group">
            <div className="bg-[#FF9F0A] p-3 rounded-xl shadow-[0_0_30px_rgba(255,159,10,0.3)] group-hover:scale-110 transition-transform">
              <CerberusLogo size={32} />
            </div>
            <div className="flex flex-col">
              <span className="font-display text-2xl font-black tracking-tighter">DASHBOARD</span>
              <span className="font-mono text-[10px] text-[#FF9F0A] uppercase tracking-[0.3em] font-black">
                Intelligence Hub
              </span>
            </div>
          </Link>

          <div className="flex items-center gap-6">
            {/* Access Timer */}
            <div className="hidden md:flex items-center gap-3 px-4 py-2 bg-zinc-900/50 border border-white/10 rounded-xl">
              <Clock size={16} className="text-[#FF9F0A]" />
              <span className="text-sm font-mono text-zinc-400">
                <span className="text-white font-black">{daysRemaining}</span> days remaining
              </span>
            </div>

            {/* User Info */}
            <span className="text-sm font-mono text-zinc-400">{user?.email}</span>

            {/* Logout Button */}
            <InteractiveButton onClick={signOut} variant="ghost" className="px-6 py-3">
              <LogOut size={18} className="mr-2" />
              Sign Out
            </InteractiveButton>
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <div className="max-w-7xl mx-auto px-6 py-12 relative z-10">
        {/* Header Section */}
        <div className="mb-12">
          <h1 className="font-display text-6xl md:text-7xl font-black mb-4 tracking-tighter">
            Product <span className="text-[#FF9F0A]">Intelligence</span>
          </h1>
          <p className="text-zinc-400 text-xl font-bold mb-8">
            Validated opportunities. Real-time market data. Zero guesswork.
          </p>

          {/* Search Bar */}
          <div className="relative mb-6">
            <Search
              className="absolute left-6 top-1/2 -translate-y-1/2 text-zinc-500"
              size={24}
            />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onKeyPress={handleKeyPress}
              placeholder="Search niches (e.g., Luxury Wellness Tech, Smart Home Devices)..."
              className="w-full bg-zinc-900/50 border-2 border-white/10 rounded-2xl pl-16 pr-6 py-6 text-lg text-white placeholder:text-zinc-600 focus:outline-none focus:border-[#FF9F0A] transition-colors"
            />
            <InteractiveButton
              onClick={() => handleSearch()}
              isLoading={isLoading}
              className="absolute right-3 top-1/2 -translate-y-1/2 px-8 py-4"
            >
              Search
            </InteractiveButton>
          </div>

          {/* Filter Tabs */}
          <div className="flex items-center gap-4 flex-wrap">
            <div className="flex items-center gap-2 text-zinc-500 font-mono text-sm uppercase tracking-widest">
              <Filter size={16} />
              Filters:
            </div>
            {[
              { id: 'all', label: 'All Products' },
              { id: 'high-growth', label: 'High Growth (70%+)' },
              { id: 'low-saturation', label: 'Low Saturation' },
            ].map((filter) => (
              <button
                key={filter.id}
                onClick={() => setSelectedFilter(filter.id as any)}
                className={`px-6 py-3 rounded-xl font-mono text-xs uppercase tracking-widest font-black transition-all ${
                  selectedFilter === filter.id
                    ? 'bg-[#FF9F0A] text-black'
                    : 'bg-zinc-900/50 border border-white/10 text-zinc-400 hover:border-[#FF9F0A]/50 hover:text-white'
                }`}
              >
                {filter.label}
              </button>
            ))}
          </div>
        </div>

        {/* Error State */}
        {error && (
          <div className="bg-red-500/10 border-2 border-red-500/50 rounded-2xl p-6 mb-8">
            <p className="text-red-400 font-mono text-lg">{error}</p>
          </div>
        )}

        {/* Loading State */}
        {isLoading && (
          <div className="py-32 flex flex-col items-center">
            <div className="relative mb-8 scale-[1.5]">
              <div className="w-24 h-24 border-6 border-white/5 border-t-[#FF9F0A] rounded-full animate-spin" />
              <div className="absolute inset-0 flex items-center justify-center">
                <Cpu size={32} className="text-[#FF9F0A] animate-pulse" />
              </div>
            </div>
            <h3 className="font-display text-3xl font-black text-white uppercase tracking-widest mb-4">
              Neural Reconstruction
            </h3>
            <p className="font-mono text-sm uppercase tracking-[0.8em] text-[#FF9F0A] font-black animate-pulse">
              Analyzing Market Intelligence...
            </p>
          </div>
        )}

        {/* Products Grid */}
        {!isLoading && filteredProducts.length > 0 && (
          <>
            <div className="mb-6 flex items-center justify-between">
              <p className="text-zinc-400 font-mono text-sm">
                <span className="text-white font-black">{filteredProducts.length}</span>{' '}
                {filteredProducts.length === 1 ? 'product' : 'products'} found
              </p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-8">
              {filteredProducts.map((product) => (
                <ProductCard key={product.id} product={product} />
              ))}
            </div>
          </>
        )}

        {/* Empty State */}
        {!isLoading && !error && filteredProducts.length === 0 && products.length === 0 && (
          <div className="text-center py-32">
            <div className="inline-block bg-zinc-900/50 border-2 border-white/10 rounded-3xl p-12">
              <h3 className="font-display text-3xl font-black text-white mb-4">
                Start Your Search
              </h3>
              <p className="text-zinc-400 font-mono mb-8 max-w-md mx-auto">
                Enter a niche or product category to discover validated opportunities with
                comprehensive market intelligence.
              </p>
            </div>
          </div>
        )}

        {/* No Results State */}
        {!isLoading && filteredProducts.length === 0 && products.length > 0 && (
          <div className="text-center py-32">
            <div className="inline-block bg-zinc-900/50 border-2 border-white/10 rounded-3xl p-12">
              <h3 className="font-display text-3xl font-black text-white mb-4">
                No Matches Found
              </h3>
              <p className="text-zinc-400 font-mono mb-8">
                Try a different search term or adjust your filters.
              </p>
              <InteractiveButton
                onClick={() => {
                  setSearchQuery('');
                  setSelectedFilter('all');
                }}
                className="px-8 py-4"
              >
                Clear Filters
              </InteractiveButton>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default DashboardPage;
