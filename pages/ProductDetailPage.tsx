import React, { useState, useEffect } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, TrendingUp, ShoppingBag, Globe, DollarSign, Package, BarChart3, ExternalLink, Star, Users, Activity, Target } from 'lucide-react';
import { DetailedProduct } from '../types';
import { getDetailedProductData } from '../services/geminiService';
import { InteractiveButton } from '../components/InteractiveButton';
import { PremiumCard } from '../components/PremiumCard';
import { AnimatedBackground } from '../components/AnimatedBackground';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar, PieChart, Pie, Cell } from 'recharts';

const ProductDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [product, setProduct] = useState<DetailedProduct | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    // In a real app, we'd fetch by ID from a database
    // For now, we'll extract the niche from the ID and fetch fresh data
    if (id) {
      const niche = id.split('-').slice(0, -2).join(' ');
      loadProduct(niche);
    }
  }, [id]);

  const loadProduct = async (niche: string) => {
    setIsLoading(true);
    setError('');

    try {
      const products = await getDetailedProductData(niche);
      // Find the product matching this ID, or use the first one
      const foundProduct = products.find((p) => p.id === id) || products[0];
      setProduct(foundProduct);
    } catch (err: any) {
      setError(err.message || 'Failed to load product details');
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoading) {
    return (
      <div className="min-h-screen bg-black flex items-center justify-center">
        <div className="text-center">
          <div className="w-32 h-32 border-6 border-white/5 border-t-[#FF9F0A] rounded-full animate-spin mx-auto mb-8" />
          <p className="text-white font-mono text-lg uppercase tracking-widest">
            Loading Intelligence...
          </p>
        </div>
      </div>
    );
  }

  if (error || !product) {
    return (
      <div className="min-h-screen bg-black flex items-center justify-center p-6">
        <div className="max-w-md text-center">
          <h2 className="font-display text-3xl font-black text-white mb-4">
            Product Not Found
          </h2>
          <p className="text-zinc-400 font-mono mb-8">{error || 'Unable to load product details'}</p>
          <Link to="/dashboard">
            <InteractiveButton className="px-8 py-4">
              <ArrowLeft size={20} className="mr-2" />
              Back to Dashboard
            </InteractiveButton>
          </Link>
        </div>
      </div>
    );
  }

  const CHART_COLORS = ['#FF9F0A', '#2E5BFF', '#34D399', '#F87171'];

  // Prepare data for charts
  const platformData = product.adAnalytics.platforms.map((p) => ({
    name: p.name,
    ads: p.count,
    engagement: p.engagement,
  }));

  const competitorData = product.competitors.websites.map((w, i) => ({
    name: `Competitor ${i + 1}`,
    visitors: w.monthlyVisitors,
    revenue: w.estimatedRevenue,
  }));

  return (
    <div className="min-h-screen bg-black text-white">
      <AnimatedBackground />

      <div className="max-w-7xl mx-auto px-6 py-8 relative z-10">
        {/* Back Button */}
        <Link to="/dashboard">
          <InteractiveButton variant="ghost" className="mb-8">
            <ArrowLeft size={20} className="mr-2" />
            Back to Dashboard
          </InteractiveButton>
        </Link>

        {/* Product Header */}
        <div className="mb-12">
          <div className="flex items-start justify-between mb-6">
            <div>
              <div className="inline-block px-4 py-2 bg-[#FF9F0A]/10 border border-[#FF9F0A]/30 rounded-xl text-[#FF9F0A] font-mono text-xs uppercase tracking-widest font-black mb-4">
                Cerberus Validated
              </div>
              <h1 className="font-display text-6xl md:text-7xl font-black mb-4 tracking-tighter">
                {product.name}
              </h1>
              <p className="text-zinc-400 text-xl font-mono uppercase tracking-widest">
                {product.niche}
              </p>
            </div>
            <div className="bg-zinc-900/50 border-2 border-[#FF9F0A]/40 rounded-2xl p-8 text-center">
              <p className="text-sm font-mono text-zinc-400 uppercase mb-2">Growth Potential</p>
              <p className="text-5xl font-black text-[#FF9F0A]">{product.growthPotential}%</p>
            </div>
          </div>

          {/* Key Metrics Bar */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-6">
            {[
              { icon: <TrendingUp size={24} />, label: 'Market Saturation', value: `${product.marketSaturation}%`, color: 'text-emerald-400' },
              { icon: <DollarSign size={24} />, label: 'Price Point', value: product.pricePoint, color: 'text-[#FF9F0A]' },
              { icon: <Activity size={24} />, label: 'Profit Margin', value: `${product.metrics.profitMargin}%`, color: 'text-emerald-400' },
              { icon: <Target size={24} />, label: 'Trend', value: product.metrics.trendDirection.toUpperCase(), color: 'text-[#2E5BFF]' },
            ].map((metric, i) => (
              <PremiumCard key={i} className="p-6 border-white/20">
                <div className={`${metric.color} mb-3`}>{metric.icon}</div>
                <p className="text-xs font-mono text-zinc-500 uppercase mb-2">{metric.label}</p>
                <p className="text-2xl font-black text-white">{metric.value}</p>
              </PremiumCard>
            ))}
          </div>
        </div>

        {/* INTERCEPTION PROTOCOL - Ad Analytics */}
        <section className="mb-16">
          <h2 className="font-display text-4xl font-black mb-8 flex items-center gap-4">
            <div className="w-12 h-12 bg-[#2E5BFF]/20 border-2 border-[#2E5BFF] rounded-xl flex items-center justify-center">
              <Activity size={24} className="text-[#2E5BFF]" />
            </div>
            INTERCEPTION PROTOCOL
          </h2>

          <div className="grid md:grid-cols-2 gap-8 mb-8">
            {/* Platform Breakdown */}
            <PremiumCard className="p-8 border-white/20">
              <h3 className="font-display text-2xl font-black mb-6">Platform Distribution</h3>
              <div className="h-80">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={platformData}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#333" />
                    <XAxis dataKey="name" stroke="#999" style={{ fontSize: '12px', fontFamily: 'Space Mono' }} />
                    <YAxis stroke="#999" style={{ fontSize: '12px', fontFamily: 'Space Mono' }} />
                    <Tooltip
                      contentStyle={{ backgroundColor: '#0a0a0a', border: '2px solid #333', borderRadius: '12px', fontFamily: 'Space Mono' }}
                    />
                    <Bar dataKey="ads" fill="#FF9F0A" />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </PremiumCard>

            {/* Total Ads */}
            <PremiumCard className="p-8 border-white/20">
              <h3 className="font-display text-2xl font-black mb-6">Ad Intelligence</h3>
              <div className="space-y-6">
                <div>
                  <p className="text-sm font-mono text-zinc-500 uppercase mb-2">Total Active Ads</p>
                  <p className="text-6xl font-black text-[#FF9F0A]">{product.adAnalytics.totalAds}</p>
                </div>
                <div className="space-y-3">
                  {product.adAnalytics.platforms.map((platform, i) => (
                    <div key={i} className="flex items-center justify-between p-4 bg-zinc-900/50 border border-white/5 rounded-xl">
                      <span className="font-mono text-sm text-white font-black">{platform.name}</span>
                      <div className="flex items-center gap-4">
                        <span className="text-zinc-400 font-mono text-sm">{platform.count} ads</span>
                        <span className="text-emerald-400 font-mono text-sm">{platform.engagement}% engagement</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </PremiumCard>
          </div>

          {/* Top Performing Ads */}
          <PremiumCard className="p-8 border-white/20">
            <h3 className="font-display text-2xl font-black mb-6">Top Performing Ads</h3>
            <div className="grid md:grid-cols-3 gap-6">
              {product.adAnalytics.topPerformingAds.map((ad, i) => (
                <div key={i} className="bg-zinc-900/50 border border-white/5 rounded-2xl p-6">
                  <div className="flex items-center justify-between mb-4">
                    <span className="font-mono text-xs text-zinc-500 uppercase">{ad.platform}</span>
                    <span className="text-emerald-400 font-black text-lg">{ad.engagement}%</span>
                  </div>
                  <a
                    href={ad.link}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-[#FF9F0A] hover:text-white font-mono text-sm flex items-center gap-2 transition-colors"
                  >
                    View Ad <ExternalLink size={14} />
                  </a>
                </div>
              ))}
            </div>
          </PremiumCard>
        </section>

        {/* MARKET INTELLIGENCE - Amazon & Competitors */}
        <section className="mb-16">
          <h2 className="font-display text-4xl font-black mb-8 flex items-center gap-4">
            <div className="w-12 h-12 bg-[#FF9F0A]/20 border-2 border-[#FF9F0A] rounded-xl flex items-center justify-center">
              <ShoppingBag size={24} className="text-[#FF9F0A]" />
            </div>
            MARKET INTELLIGENCE
          </h2>

          {/* Amazon Competitors */}
          <PremiumCard className="p-8 border-white/20 mb-8">
            <div className="flex items-center justify-between mb-6">
              <h3 className="font-display text-2xl font-black">Amazon Marketplace</h3>
              <div className="text-right">
                <p className="text-sm font-mono text-zinc-500 uppercase">Est. Market Revenue</p>
                <p className="text-3xl font-black text-emerald-400">
                  ${product.amazonData.estimatedRevenue.toLocaleString()}
                </p>
              </div>
            </div>

            <div className="space-y-4">
              {product.amazonData.competitorProducts.map((comp, i) => (
                <div key={i} className="bg-zinc-900/50 border border-white/5 rounded-xl p-6 flex items-center justify-between hover:border-[#FF9F0A]/30 transition-colors">
                  <div className="flex-1">
                    <h4 className="font-bold text-white mb-2">{comp.title}</h4>
                    <div className="flex items-center gap-4 text-sm">
                      <div className="flex items-center gap-1">
                        <Star size={14} className="text-[#FF9F0A] fill-[#FF9F0A]" />
                        <span className="font-mono text-white font-black">{comp.rating}</span>
                      </div>
                      <span className="text-zinc-500 font-mono">{comp.reviews.toLocaleString()} reviews</span>
                      {comp.asin && <span className="text-zinc-600 font-mono text-xs">ASIN: {comp.asin}</span>}
                    </div>
                  </div>
                  <div className="flex items-center gap-6">
                    <div className="text-right">
                      <p className="text-2xl font-black text-white">${comp.price}</p>
                    </div>
                    <a
                      href={comp.link}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="p-3 bg-[#FF9F0A] hover:bg-white text-black rounded-xl transition-colors"
                    >
                      <ExternalLink size={18} />
                    </a>
                  </div>
                </div>
              ))}
            </div>
          </PremiumCard>

          {/* Competitor Websites */}
          <PremiumCard className="p-8 border-white/20">
            <h3 className="font-display text-2xl font-black mb-6">Competitor Analysis</h3>
            <div className="grid md:grid-cols-2 gap-6">
              {product.competitors.websites.map((site, i) => (
                <div key={i} className="bg-zinc-900/50 border border-white/5 rounded-xl p-6">
                  <div className="flex items-center gap-3 mb-4">
                    <Globe size={20} className="text-[#2E5BFF]" />
                    <a
                      href={site.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-white hover:text-[#FF9F0A] font-mono text-sm transition-colors flex items-center gap-2"
                    >
                      {site.url.replace(/^https?:\/\//, '')} <ExternalLink size={12} />
                    </a>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-xs font-mono text-zinc-500 uppercase mb-1">Monthly Visitors</p>
                      <p className="text-xl font-black text-white">{(site.monthlyVisitors / 1000).toFixed(1)}K</p>
                    </div>
                    <div>
                      <p className="text-xs font-mono text-zinc-500 uppercase mb-1">Est. Revenue</p>
                      <p className="text-xl font-black text-emerald-400">${(site.estimatedRevenue / 1000).toFixed(0)}K</p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
            <div className="mt-6 p-6 bg-[#2E5BFF]/10 border border-[#2E5BFF]/30 rounded-xl">
              <p className="text-sm font-mono text-zinc-400 mb-2">Your Market Share Opportunity</p>
              <p className="text-4xl font-black text-[#2E5BFF]">{product.competitors.marketShare}%</p>
            </div>
          </PremiumCard>
        </section>

        {/* SOURCING PROTOCOL */}
        <section className="mb-16">
          <h2 className="font-display text-4xl font-black mb-8 flex items-center gap-4">
            <div className="w-12 h-12 bg-emerald-500/20 border-2 border-emerald-500 rounded-xl flex items-center justify-center">
              <Package size={24} className="text-emerald-400" />
            </div>
            SOURCING PROTOCOL
          </h2>

          <div className="grid md:grid-cols-2 gap-8">
            <PremiumCard className="p-8 border-white/20">
              <h3 className="font-display text-2xl font-black mb-6">Alibaba Sourcing</h3>
              <div className="space-y-6">
                <div>
                  <p className="text-sm font-mono text-zinc-500 uppercase mb-2">Supplier</p>
                  <p className="text-lg font-bold text-white">{product.sourcing.supplier}</p>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-xs font-mono text-zinc-500 uppercase mb-1">MOQ</p>
                    <p className="text-2xl font-black text-white">{product.sourcing.moq} units</p>
                  </div>
                  <div>
                    <p className="text-xs font-mono text-zinc-500 uppercase mb-1">Unit Price</p>
                    <p className="text-2xl font-black text-emerald-400">${product.sourcing.unitPrice}</p>
                  </div>
                </div>
                <div>
                  <p className="text-sm font-mono text-zinc-500 uppercase mb-2">Shipping Time</p>
                  <p className="text-lg font-bold text-white">{product.sourcing.shippingTime}</p>
                </div>
                <a
                  href={product.alibabaLink}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="block w-full"
                >
                  <InteractiveButton className="w-full px-8 py-4">
                    View on Alibaba <ExternalLink size={18} className="ml-2" />
                  </InteractiveButton>
                </a>
              </div>
            </PremiumCard>

            {/* Profit Calculator */}
            <PremiumCard className="p-8 border-white/20 bg-emerald-500/5">
              <h3 className="font-display text-2xl font-black mb-6 text-emerald-400">Profit Projection</h3>
              <div className="space-y-4">
                <div className="flex justify-between items-center p-4 bg-zinc-900/50 rounded-xl">
                  <span className="font-mono text-sm text-zinc-400">Cost Per Unit</span>
                  <span className="font-black text-lg text-white">${product.sourcing.unitPrice}</span>
                </div>
                <div className="flex justify-between items-center p-4 bg-zinc-900/50 rounded-xl">
                  <span className="font-mono text-sm text-zinc-400">Selling Price</span>
                  <span className="font-black text-lg text-white">{product.pricePoint}</span>
                </div>
                <div className="flex justify-between items-center p-4 bg-emerald-500/10 border border-emerald-500/30 rounded-xl">
                  <span className="font-mono text-sm text-emerald-400">Profit Margin</span>
                  <span className="font-black text-2xl text-emerald-400">{product.metrics.profitMargin}%</span>
                </div>
                <div className="flex justify-between items-center p-4 bg-zinc-900/50 rounded-xl">
                  <span className="font-mono text-sm text-zinc-400">Break-Even Units</span>
                  <span className="font-black text-lg text-white">{product.metrics.breakEvenUnits}</span>
                </div>
              </div>
            </PremiumCard>
          </div>
        </section>

        {/* VALIDATION METRICS */}
        <section className="mb-16">
          <h2 className="font-display text-4xl font-black mb-8 flex items-center gap-4">
            <div className="w-12 h-12 bg-[#FF9F0A]/20 border-2 border-[#FF9F0A] rounded-xl flex items-center justify-center">
              <BarChart3 size={24} className="text-[#FF9F0A]" />
            </div>
            VALIDATION METRICS
          </h2>

          <div className="grid md:grid-cols-3 gap-8">
            <PremiumCard className="p-8 border-white/20">
              <h3 className="font-display text-xl font-black mb-4">Search Volume</h3>
              <p className="text-5xl font-black text-white mb-2">{(product.metrics.searchVolume / 1000).toFixed(1)}K</p>
              <p className="text-sm font-mono text-zinc-500 uppercase">Monthly searches</p>
            </PremiumCard>

            <PremiumCard className="p-8 border-white/20">
              <h3 className="font-display text-xl font-black mb-4">Seasonality</h3>
              <p className="text-2xl font-black text-white mb-2">{product.metrics.seasonality}</p>
              <p className="text-sm font-mono text-zinc-500 uppercase">Pattern identified</p>
            </PremiumCard>

            <PremiumCard className="p-8 border-white/20 bg-[#FF9F0A]/5">
              <h3 className="font-display text-xl font-black mb-4">Recommended Strategy</h3>
              <p className="text-sm font-mono text-white leading-relaxed">{product.recommendedStrategy}</p>
            </PremiumCard>
          </div>

          {/* Justification */}
          <PremiumCard className="p-8 border-white/20 mt-8">
            <h3 className="font-display text-2xl font-black mb-4">Why Cerberus Flagged This</h3>
            <p className="text-lg text-zinc-300 leading-relaxed">{product.justification}</p>
          </PremiumCard>
        </section>

        {/* CTA */}
        <div className="text-center">
          <Link to="/dashboard">
            <InteractiveButton className="px-12 py-6 text-lg">
              <ArrowLeft size={20} className="mr-2" />
              Back to Dashboard
            </InteractiveButton>
          </Link>
        </div>
      </div>
    </div>
  );
};

export default ProductDetailPage;
