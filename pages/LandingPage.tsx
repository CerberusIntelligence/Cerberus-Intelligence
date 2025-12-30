
import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import {
  ShieldCheck,
  TrendingUp,
  ChevronRight,
  Lock,
  Globe,
  ExternalLink,
  Activity,
  Terminal,
  Zap,
  Target,
  Search,
  Cpu,
  Unlock
} from 'lucide-react';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer
} from 'recharts';
import { analyzeSocialAds } from '../services/geminiService';
import { ProductInsight, MarketMetrics } from '../types';
import { PremiumCard } from '../components/PremiumCard';
import { InteractiveButton } from '../components/InteractiveButton';
import { CerberusLogo } from '../components/CerberusLogo';
import { AnimatedBackground } from '../components/AnimatedBackground';

const MOCK_DATA: MarketMetrics[] = [
  { month: '01', demand: 45, competition: 30 },
  { month: '02', demand: 52, competition: 32 },
  { month: '03', demand: 48, competition: 40 },
  { month: '04', demand: 61, competition: 35 },
  { month: '05', demand: 55, competition: 38 },
  { month: '06', demand: 67, competition: 42 },
  { month: '07', demand: 85, competition: 45 },
];

const LandingPage: React.FC = () => {
  const [nicheInput, setNicheInput] = useState('');
  const [results, setResults] = useState<(ProductInsight & { adCount?: number })[]>([]);
  const [isAnalyzing, setIsAnalyzing] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const [unlockedIndices, setUnlockedIndices] = useState<Set<number>>(new Set());

  useEffect(() => {
    const handleScroll = () => setScrolled(window.scrollY > 40);
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  const handleAnalyze = async () => {
    if (!nicheInput.trim()) return;
    setIsAnalyzing(true);
    setUnlockedIndices(new Set()); // Reset unlocks on new scan
    try {
      const data = await analyzeSocialAds(nicheInput);
      setResults(data);
      document.getElementById('tool-section')?.scrollIntoView({ behavior: 'smooth' });
    } catch (error) {
      console.error(error);
      const errorMessage = error instanceof Error ? error.message : 'Unknown error occurred';
      alert(`Cerberus Protocol Error: ${errorMessage}`);
      setResults([]);
    } finally {
      setIsAnalyzing(false);
    }
  };

  const handleUnlock = (idx: number) => {
    setUnlockedIndices(prev => {
      const next = new Set(prev);
      next.add(idx);
      return next;
    });
  };

  return (
    <div className="min-h-screen text-zinc-100 font-sans tracking-tight bg-black">
      <AnimatedBackground />
      
      {/* HUD Navigation */}
      <nav className={`fixed top-0 left-0 right-0 z-50 transition-all duration-700 ${scrolled ? 'py-5 custom-blur bg-[#050505f2] border-b border-white/[0.15] shadow-[0_20px_50px_rgba(0,0,0,0.8)]' : 'py-10 bg-transparent'}`}>
        <div className="max-w-7xl mx-auto px-10 flex items-center justify-between">
          <div className="flex items-center gap-6 group cursor-pointer">
            <div className="bg-[#FF9F0A] p-3 rounded-xl text-black shadow-[0_0_30px_rgba(255,159,10,0.5)] transition-all group-hover:scale-110">
              <CerberusLogo size={32} />
            </div>
            <div className="flex flex-col">
              <span className="font-display text-3xl font-black tracking-tighter text-white leading-none">CERBERUS PROTOCOL</span>
              <span className="font-mono text-[11px] text-[#FF9F0A] uppercase tracking-[0.5em] font-black">Secure Intelligence Hub</span>
            </div>
          </div>
          
          <div className="hidden md:flex items-center gap-12 font-mono text-[12px] font-black tracking-[0.25em] uppercase">
            <a href="#tool-section" className="text-zinc-300 hover:text-[#FF9F0A] transition-all relative group">
              Interception
              <span className="absolute -bottom-2 left-0 w-0 h-0.5 bg-[#FF9F0A] transition-all group-hover:w-full"></span>
            </a>
            <div className="h-6 w-px bg-white/20" />
            <Link to="/login">
              <InteractiveButton
                className="px-8 py-3.5 rounded-xl shadow-[0_0_40px_rgba(255,159,10,0.3)] bg-[#FF9F0A] text-black hover:bg-white hover:text-black"
              >
                System Access
              </InteractiveButton>
            </Link>
          </div>
        </div>
      </nav>

      {/* Hero: Bold, Massive & High Contrast */}
      <section className="relative pt-72 pb-48 min-h-screen flex flex-col justify-center">
        <div className="max-w-7xl mx-auto px-10 text-center relative z-10">
          <div className="inline-flex items-center gap-5 px-8 py-3 bg-zinc-900/80 border-2 border-[#FF9F0A]/40 rounded-full text-[#FF9F0A] font-mono text-[12px] font-black uppercase tracking-[0.4em] mb-16 backdrop-blur-2xl shadow-3xl">
            <div className="w-3 h-3 rounded-full bg-[#FF9F0A] animate-pulse shadow-[0_0_15px_#FF9F0A]" />
            Node-01 Fully Operational: Global Stream Intercept Enabled
          </div>
          
          <h1 className="font-display text-8xl md:text-[11rem] font-black text-white mb-12 tracking-tighter uppercase leading-[0.75] drop-shadow-[0_20px_50px_rgba(0,0,0,0.7)]">
            Guard Your <br />
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-[#FF9F0A] via-amber-300 to-[#FF9F0A] animate-pulse">Conviction.</span>
          </h1>
          
          <p className="max-w-4xl mx-auto text-zinc-100 text-2xl md:text-3xl mb-20 font-semibold leading-relaxed drop-shadow-lg">
            Cerberus Protocol intercepts emerging growth vectors from <span className="text-white font-black underline decoration-[#FF9F0A] decoration-4 underline-offset-8">4.2 Billion data points</span> daily across dark social subnets.
          </p>
          
          <div className="flex flex-col sm:flex-row items-center justify-center gap-10">
            <InteractiveButton 
              onClick={() => document.getElementById('tool-section')?.scrollIntoView({ behavior: 'smooth' })}
              className="px-16 py-8 text-lg rounded-2xl shadow-[0_30px_60px_rgba(255,159,10,0.3)]"
            >
              Initiate Scan <ChevronRight size={24} className="inline ml-2" />
            </InteractiveButton>
            
            <div className="flex items-center gap-12 text-[14px] font-mono text-zinc-300 uppercase tracking-[0.5em] font-black">
              <span className="flex items-center gap-4 hover:text-[#2E5BFF] transition-colors"><Globe size={20} className="text-[#2E5BFF]" /> 1.4k Nodes</span>
              <span className="flex items-center gap-4 hover:text-[#FF9F0A] transition-colors"><Zap size={20} className="text-[#FF9F0A]" /> 12ms Latency</span>
            </div>
          </div>
        </div>
      </section>

      {/* Logic Ribbon: Massive Anchors */}
      <section className="border-y-2 border-white/[0.2] bg-zinc-950/90 backdrop-blur-2xl relative z-20">
        <div className="max-w-7xl mx-auto px-10 py-16 flex flex-wrap md:flex-nowrap gap-16 justify-between">
          {[
            { icon: <Globe size={32} />, label: "Inflow Throughput", val: "220M/s", color: "text-[#2E5BFF]" },
            { icon: <ShieldCheck size={32} />, label: "Security Protocol", val: "Active", color: "text-[#FF9F0A]" },
            { icon: <Target size={32} />, label: "Detection Accuracy", val: "99.2%", color: "text-emerald-400" },
            { icon: <Zap size={32} />, label: "Neural Velocity", val: "Sub-ms", color: "text-amber-300" }
          ].map((f, i) => (
            <div key={i} className="flex items-center gap-8 group hover:scale-105 transition-all duration-300">
              <div className={`${f.color} bg-zinc-900 border-2 border-white/10 p-5 rounded-2xl group-hover:border-white/40 shadow-2xl transition-all`}>
                {f.icon}
              </div>
              <div className="flex flex-col">
                <span className="text-[12px] uppercase tracking-[0.4em] text-zinc-500 font-mono font-black mb-2">{f.label}</span>
                <span className="text-3xl font-black text-white font-display leading-tight">{f.val}</span>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* Interceptor Dashboard: Full-Width Command Terminal Under Text */}
      <section id="tool-section" className="py-56 bg-black relative">
        <div className="max-w-7xl mx-auto px-10 relative z-10">
          <div className="flex flex-col items-center lg:items-start mb-24">
            <h2 className="font-display text-6xl md:text-8xl font-black text-white uppercase tracking-tighter mb-8 leading-none">
              The <span className="text-[#FF9F0A]">Interceptor.</span>
            </h2>
            <p className="text-zinc-100 text-2xl font-bold leading-relaxed max-w-4xl text-center lg:text-left mb-20">
              Initiate deep-layer subnet scanning. Define your target niche string for immediate neural interception.
            </p>
            
            <div className="w-full max-w-5xl relative group mx-auto lg:mx-0">
              {/* Terminal Header Decor */}
              <div className="absolute -top-7 left-0 right-0 flex items-center justify-between px-6 py-1 bg-zinc-900 border-t-2 border-x-2 border-white/10 rounded-t-xl font-mono text-[9px] font-black uppercase tracking-[0.4em] text-zinc-500">
                <div className="flex items-center gap-3">
                  <div className="w-1.5 h-1.5 rounded-full bg-red-500/50" />
                  <div className="w-1.5 h-1.5 rounded-full bg-[#FF9F0A]/50" />
                  <div className="w-1.5 h-1.5 rounded-full bg-emerald-500/50" />
                  <span className="ml-2">COMMAND_ENTRY_NODE:01</span>
                </div>
                <div className="flex items-center gap-3">
                  <Activity size={10} className="animate-pulse" />
                  <span>SECURE_SESSION_ACTIVE</span>
                </div>
              </div>

              <div className={`relative transition-all duration-500 ${isAnalyzing ? 'shadow-[0_0_60px_rgba(255,159,10,0.2)]' : 'shadow-4xl'}`}>
                <div className="absolute inset-y-0 left-0 pl-10 flex items-center pointer-events-none">
                  <Terminal className={`${isAnalyzing ? 'text-[#FF9F0A] animate-pulse' : 'text-[#FF9F0A]'}`} size={28} />
                </div>
                
                <input 
                  type="text" 
                  placeholder="ENTER_NICHE: e.g. Luxury Wellness Tech"
                  value={nicheInput}
                  onChange={(e) => setNicheInput(e.target.value)}
                  className={`
                    w-full bg-black border-2 rounded-b-2xl rounded-tr-2xl py-11 pl-24 pr-64 text-2xl font-mono text-white placeholder:text-zinc-800
                    focus:outline-none transition-all duration-500
                    ${isAnalyzing 
                      ? 'border-[#FF9F0A] ring-8 ring-[#FF9F0A]/5 cursor-wait' 
                      : 'border-white/10 focus:border-[#FF9F0A] focus:ring-12 focus:ring-[#FF9F0A]/10'
                    }
                  `}
                  disabled={isAnalyzing}
                  onKeyDown={(e) => e.key === 'Enter' && handleAnalyze()}
                />
                
                <button 
                  onClick={handleAnalyze}
                  disabled={isAnalyzing}
                  className={`
                    absolute right-4 top-4 bottom-4 px-12 rounded-xl font-black uppercase tracking-[0.2em] text-[13px]
                    transition-all duration-500 flex items-center gap-4
                    ${isAnalyzing 
                      ? 'bg-white text-black cursor-wait px-14' 
                      : 'bg-[#FF9F0A] text-black hover:bg-white hover:scale-105 active:scale-95 shadow-[0_10px_30px_rgba(0,0,0,0.5)]'
                    }
                  `}
                >
                  {isAnalyzing ? (
                    <>
                      <Cpu size={22} className="animate-spin" />
                      SYNCING...
                    </>
                  ) : (
                    <>
                      <Search size={22} />
                      EXECUTE PROTOCOL
                    </>
                  )}
                </button>
              </div>

              {/* Terminal Footer Decor */}
              <div className="mt-4 flex items-center justify-between font-mono text-[10px] font-black uppercase tracking-[0.3em] text-zinc-600 px-2">
                <div className="flex items-center gap-4">
                  <span className="text-[#FF9F0A] animate-pulse">●</span>
                  <span>System_Auth: Admin_Root</span>
                </div>
                <div className="flex items-center gap-6">
                  <span>ENCRYPTION: AES-256</span>
                  <span>v1.02.4-STABLE</span>
                </div>
              </div>
            </div>
          </div>

          {isAnalyzing ? (
            <div className="py-64 flex flex-col items-center">
              <div className="relative mb-16 scale-[2]">
                <div className="w-32 h-32 border-6 border-white/5 border-t-[#FF9F0A] rounded-full animate-spin" />
                <div className="absolute inset-0 flex items-center justify-center">
                  <CerberusLogo size={48} className="text-[#FF9F0A] animate-pulse" />
                </div>
              </div>
              <h3 className="font-display text-4xl font-black text-white uppercase tracking-widest mb-6">Neural Reconstruction</h3>
              <p className="font-mono text-[14px] uppercase tracking-[0.8em] text-[#FF9F0A] font-black animate-pulse">Filtering Encryption Barriers...</p>
            </div>
          ) : results.length > 0 ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-12">
              {results.map((product, idx) => {
                const isUnlocked = unlockedIndices.has(idx);
                return (
                  <PremiumCard key={idx} className="flex flex-col group min-h-[600px] border-white/20 p-10">
                    <div className="flex justify-between items-start mb-10">
                      <div className="flex flex-col">
                        <span className="text-[12px] font-mono text-[#2E5BFF] font-black tracking-widest">SIGNAL: {idx}XP-ALPHA</span>
                        <span className="text-[11px] font-mono text-zinc-500 uppercase font-bold">Node-Certified</span>
                      </div>
                      <div className="bg-black px-6 py-3 rounded-2xl border-2 border-[#FF9F0A]/40 text-[18px] font-black text-[#FF9F0A] flex items-center gap-3 shadow-3xl">
                        <TrendingUp size={20} /> {product.growthPotential}%
                      </div>
                    </div>
                    
                    <h3 className={`font-display text-3xl font-black text-white mb-4 transition-all duration-700 leading-tight ${!isUnlocked ? 'blur-xl select-none opacity-40 grayscale' : 'group-hover:text-[#FF9F0A]'}`}>
                      {product.name}
                    </h3>
                    
                    <div className="inline-block self-start px-4 py-1.5 bg-white/5 border-2 border-white/10 rounded-lg text-[11px] font-mono text-zinc-100 uppercase tracking-widest font-black mb-10">
                      {product.niche}
                    </div>
                    
                    <div className="relative flex-grow bg-black border-2 border-white/5 rounded-3xl p-8 mb-10 flex flex-col justify-center items-center overflow-hidden group/payload">
                      <div className={`absolute inset-0 p-6 opacity-[0.1] blur-[1px] font-mono text-[11px] leading-relaxed text-zinc-300 transition-all duration-700 ${!isUnlocked ? 'blur-lg' : ''}`}>
                        {product.justification}
                      </div>
                      
                      <div className="relative z-10 flex flex-col items-center text-center">
                        {!isUnlocked ? (
                          <>
                            <div className="w-20 h-20 bg-zinc-900 border-3 border-[#FF9F0A]/30 rounded-full flex items-center justify-center mb-8 shadow-4xl group-hover/payload:border-[#FF9F0A] transition-all">
                              <Lock size={36} className="text-[#FF9F0A]" />
                            </div>
                            <span className="text-[12px] uppercase tracking-[0.5em] text-white font-mono font-black mb-8">Protocol Locked</span>
                            <InteractiveButton 
                              onClick={() => handleUnlock(idx)}
                              className="text-[12px] px-10 py-4 rounded-xl shadow-2xl shadow-amber-500/20 hover:scale-110"
                            >
                              Unlock Intel $150
                            </InteractiveButton>
                          </>
                        ) : (
                          <div className="flex flex-col items-center animate-in fade-in zoom-in duration-500">
                             <div className="w-20 h-20 bg-emerald-500/10 border-3 border-emerald-500/40 rounded-full flex items-center justify-center mb-8 shadow-4xl">
                              <Unlock size={36} className="text-emerald-400" />
                            </div>
                            <span className="text-[12px] uppercase tracking-[0.5em] text-emerald-400 font-mono font-black mb-4">Intel Decrypted</span>
                            <p className="text-zinc-400 font-mono text-[10px] uppercase tracking-[0.2em]">{product.adCount} Active Intercepts</p>
                          </div>
                        )}
                      </div>
                    </div>

                    <div className="flex items-center justify-between pt-10 border-t-2 border-white/10 mt-auto">
                      <div className="flex flex-col">
                        <span className="text-[11px] font-mono text-zinc-500 uppercase font-black mb-1.5">Target Entry</span>
                        <span className="text-3xl font-black text-white font-display">{product.pricePoint}</span>
                      </div>
                      <button className="p-5 bg-zinc-900 border-2 border-white/10 rounded-2xl hover:bg-[#FF9F0A] hover:text-black transition-all shadow-3xl">
                        <ExternalLink size={24} />
                      </button>
                    </div>
                  </PremiumCard>
                );
              })}
            </div>
          ) : (
            <div className="py-48 text-center border-6 border-dashed border-white/5 rounded-[4rem] bg-zinc-900/10">
              <Activity size={64} className="text-zinc-800 mx-auto mb-10 animate-pulse" />
              <h4 className="font-display text-zinc-400 font-black text-3xl uppercase tracking-widest mb-6">Cerberus Idle</h4>
              <p className="text-zinc-600 text-lg font-mono uppercase tracking-[0.4em] font-black italic">Awaiting Protocol Initialization... Level 01</p>
            </div>
          )}
        </div>
      </section>

      {/* Network Pulse: Massive Stats */}
      <section className="py-56 border-t-2 border-white/10 bg-[#030303]">
        <div className="max-w-7xl mx-auto px-10">
          <div className="grid lg:grid-cols-2 gap-40 items-center">
            <div>
              <div className="inline-block px-8 py-3 bg-emerald-500/10 border-2 border-emerald-500/40 rounded-full text-emerald-400 font-mono text-[12px] font-black uppercase tracking-[0.5em] mb-12">
                Real-Time Data Extraction
              </div>
              <h2 className="font-display text-7xl md:text-9xl font-black text-white uppercase tracking-tighter mb-12 leading-none">The Neural <span className="text-[#2E5BFF]">Pulse.</span></h2>
              <p className="text-zinc-100 text-2xl font-bold leading-relaxed mb-20 drop-shadow-md">
                Protocol-01 identifies exactly where quiet market interest evolves into mass demand. Our metrics are un-falsifiable and retrieved in real-time from the source.
              </p>
              
              <div className="grid grid-cols-2 gap-10">
                {[
                  { l: "Global Intercepts", v: "220M+", color: "bg-[#FF9F0A]" },
                  { l: "System Latency", v: "12ms", color: "bg-[#2E5BFF]" },
                  { l: "Intercept Accuracy", v: "99.2%", color: "bg-emerald-500" },
                  { l: "Uptime Protocol", v: "99.9%", color: "bg-white" }
                ].map((s, i) => (
                  <div key={i} className="p-10 bg-zinc-900/40 border-2 border-white/10 rounded-[2.5rem] shadow-4xl hover:border-white/30 transition-all group">
                    <p className="text-[13px] uppercase tracking-[0.4em] text-zinc-500 mb-4 font-mono font-black group-hover:text-white transition-colors">{s.l}</p>
                    <p className="text-5xl font-black text-white font-display mb-8">{s.v}</p>
                    <div className="w-full h-2.5 bg-zinc-800 rounded-full overflow-hidden">
                      <div className={`h-full rounded-full ${s.color} shadow-[0_0_20px_rgba(255,255,255,0.3)]`} style={{ width: '85%' }} />
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className="h-[600px] bg-black border-3 border-white/10 rounded-[4rem] p-16 relative shadow-[0_60px_120px_rgba(0,0,0,0.95)] overflow-hidden">
               <div className="absolute top-10 left-12 flex items-center gap-5">
                <div className="w-4 h-4 rounded-full bg-[#FF9F0A] animate-pulse shadow-[0_0_15px_#FF9F0A]" />
                <span className="text-[14px] font-mono text-zinc-300 font-black uppercase tracking-[0.5em]">SIGNAL_OSCILLATION_LIVE</span>
              </div>
              
              <div className="w-full h-full pt-20">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={MOCK_DATA}>
                    <defs>
                      <linearGradient id="colorPulse" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#FF9F0A" stopOpacity={0.4}/>
                        <stop offset="95%" stopColor="#FF9F0A" stopOpacity={0}/>
                      </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="6 6" vertical={false} stroke="#ffffff15" />
                    <XAxis dataKey="month" axisLine={false} tickLine={false} tick={{fill: '#999', fontSize: 14, fontWeight: '900', fontFamily: 'Space Mono'}} />
                    <YAxis axisLine={false} tickLine={false} tick={{fill: '#999', fontSize: 14, fontWeight: '900', fontFamily: 'Space Mono'}} />
                    <Tooltip 
                      contentStyle={{backgroundColor: '#0a0a0a', borderRadius: '24px', border: '2px solid #333', color: '#fff', fontWeight: '900', fontFamily: 'Space Mono'}}
                    />
                    <Area type="monotone" dataKey="demand" stroke="#FF9F0A" strokeWidth={8} fill="url(#colorPulse)" />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Footer: Dominant Brand Terminal */}
      <footer className="py-32 bg-black border-t-2 border-white/10 relative z-10">
        <div className="max-w-7xl mx-auto px-10">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-24 mb-32">
            <div className="col-span-1 lg:col-span-2">
              <div className="flex items-center gap-8 mb-12">
                <div className="bg-[#FF9F0A] p-4 rounded-2xl text-black shadow-3xl">
                  <CerberusLogo size={56} />
                </div>
                <div className="flex flex-col">
                  <span className="text-4xl font-black text-white font-display tracking-tighter">CERBERUS PROTOCOL</span>
                  <span className="text-[14px] text-zinc-500 font-mono uppercase tracking-[0.6em] font-black">Elite Defense Matrix</span>
                </div>
              </div>
              <p className="max-w-xl text-zinc-300 text-2xl font-bold leading-relaxed mb-16 italic">
                The gatekeeper of high-conviction capital. Protecting your assets through deterministic neural market interception.
              </p>
            </div>
            
            <div>
              <h4 className="font-display text-white text-2xl font-black uppercase mb-12 tracking-[0.2em]">Matrix</h4>
              <ul className="space-y-8 font-mono text-[15px] font-black text-zinc-400 uppercase tracking-widest">
                <li><a href="#" className="hover:text-[#FF9F0A] transition-colors flex items-center gap-4">Deep_Miner <ChevronRight size={18} /></a></li>
                <li><a href="#" className="hover:text-[#FF9F0A] transition-colors flex items-center gap-4">Intercept_v2 <ChevronRight size={18} /></a></li>
                <li><a href="#" className="hover:text-[#FF9F0A] transition-colors flex items-center gap-4">Defense_Root <ChevronRight size={18} /></a></li>
              </ul>
            </div>
            
            <div>
              <h4 className="font-display text-white text-2xl font-black uppercase mb-12 tracking-[0.2em]">Vault</h4>
              <ul className="space-y-8 font-mono text-[15px] font-black text-zinc-400 uppercase tracking-widest">
                <li><a href="#" className="hover:text-[#2E5BFF] transition-colors">Secure_API</a></li>
                <li><a href="#" className="hover:text-[#2E5BFF] transition-colors">Node_Identity</a></li>
                <li><a href="#" className="hover:text-[#2E5BFF] transition-colors">Protocol_Doc</a></li>
              </ul>
            </div>
          </div>
          
          <div className="pt-16 border-t border-white/10 flex flex-col md:flex-row justify-between items-center gap-12 font-mono text-[12px] uppercase tracking-[0.4em] font-black text-zinc-700">
            <p>&copy; 2025 CERBERUS PROTOCOL // SECURE_NODE: ALPHA-X86</p>
            <div className="flex gap-16">
              <a href="#" className="hover:text-white transition-colors">Legal_Disclosure</a>
              <a href="#" className="hover:text-white transition-colors">Privacy_Matrix</a>
              <a href="#" className="hover:text-[#FF9F0A] transition-colors">Terms_of_Protocol</a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
};

export default LandingPage;
