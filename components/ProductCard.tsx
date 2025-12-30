import React from 'react';
import { Link } from 'react-router-dom';
import { TrendingUp, ExternalLink, ShoppingCart, Activity } from 'lucide-react';
import { DetailedProduct } from '../types';
import { PremiumCard } from './PremiumCard';

interface ProductCardProps {
  product: DetailedProduct;
}

export const ProductCard: React.FC<ProductCardProps> = ({ product }) => {
  // Determine trend color based on direction
  const getTrendColor = () => {
    if (product.metrics.trendDirection === 'up') return 'text-emerald-400';
    if (product.metrics.trendDirection === 'down') return 'text-red-400';
    return 'text-zinc-400';
  };

  // Get saturation level indicator
  const getSaturationLevel = () => {
    if (product.marketSaturation < 30) return { label: 'Low', color: 'bg-emerald-500' };
    if (product.marketSaturation < 60) return { label: 'Medium', color: 'bg-[#FF9F0A]' };
    return { label: 'High', color: 'bg-red-500' };
  };

  const saturation = getSaturationLevel();

  return (
    <Link to={`/product/${product.id}`} className="block">
      <PremiumCard className="flex flex-col h-full group cursor-pointer border-white/20 p-8 hover:border-[#FF9F0A]/50 transition-all duration-500">
        {/* Header */}
        <div className="flex justify-between items-start mb-6">
          <div className="flex flex-col">
            <span className="text-[11px] font-mono text-[#2E5BFF] font-black tracking-widest uppercase">
              VALIDATED
            </span>
            <span className="text-[10px] font-mono text-zinc-500 uppercase font-bold">
              Cerberus Protocol
            </span>
          </div>
          <div className="bg-black px-5 py-2.5 rounded-xl border-2 border-[#FF9F0A]/40 text-[16px] font-black text-[#FF9F0A] flex items-center gap-2 shadow-2xl">
            <TrendingUp size={18} />
            {product.growthPotential}%
          </div>
        </div>

        {/* Product Name */}
        <h3 className="font-display text-2xl font-black text-white mb-3 leading-tight group-hover:text-[#FF9F0A] transition-colors">
          {product.name}
        </h3>

        {/* Niche Badge */}
        <div className="inline-block self-start px-4 py-1.5 bg-white/5 border-2 border-white/10 rounded-lg text-[10px] font-mono text-zinc-100 uppercase tracking-widest font-black mb-6">
          {product.niche}
        </div>

        {/* Metrics Grid */}
        <div className="grid grid-cols-2 gap-4 mb-6">
          <div className="bg-zinc-900/50 border border-white/5 rounded-xl p-4">
            <p className="text-[10px] font-mono text-zinc-500 uppercase mb-1">Ads Active</p>
            <p className="text-2xl font-black text-white">{product.adAnalytics.totalAds}</p>
          </div>
          <div className="bg-zinc-900/50 border border-white/5 rounded-xl p-4">
            <p className="text-[10px] font-mono text-zinc-500 uppercase mb-1">Profit Margin</p>
            <p className="text-2xl font-black text-emerald-400">{product.metrics.profitMargin}%</p>
          </div>
        </div>

        {/* Market Saturation Indicator */}
        <div className="mb-6">
          <div className="flex justify-between items-center mb-2">
            <span className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">
              Market Saturation
            </span>
            <span className={`text-[11px] font-mono font-black ${saturation.color === 'bg-emerald-500' ? 'text-emerald-400' : saturation.color === 'bg-[#FF9F0A]' ? 'text-[#FF9F0A]' : 'text-red-400'}`}>
              {saturation.label}
            </span>
          </div>
          <div className="w-full h-2 bg-zinc-800 rounded-full overflow-hidden">
            <div
              className={`h-full rounded-full ${saturation.color} transition-all duration-700`}
              style={{ width: `${product.marketSaturation}%` }}
            />
          </div>
        </div>

        {/* Price & Trend */}
        <div className="flex items-center justify-between pt-6 border-t-2 border-white/10 mt-auto">
          <div className="flex flex-col">
            <span className="text-[10px] font-mono text-zinc-500 uppercase font-black mb-1">
              Price Point
            </span>
            <span className="text-2xl font-black text-white font-display">
              {product.pricePoint}
            </span>
          </div>
          <div className="flex flex-col items-end">
            <span className="text-[10px] font-mono text-zinc-500 uppercase font-black mb-1">
              Trend
            </span>
            <div className="flex items-center gap-2">
              <Activity size={18} className={getTrendColor()} />
              <span className={`text-sm font-mono font-black uppercase ${getTrendColor()}`}>
                {product.metrics.trendDirection}
              </span>
            </div>
          </div>
        </div>

        {/* Hover CTA */}
        <div className="mt-6 flex items-center justify-between opacity-0 group-hover:opacity-100 transition-opacity duration-300">
          <span className="text-xs font-mono text-[#FF9F0A] uppercase tracking-widest font-black">
            View Full Intel →
          </span>
          <ExternalLink size={16} className="text-[#FF9F0A]" />
        </div>
      </PremiumCard>
    </Link>
  );
};
