
export interface ProductInsight {
  name: string;
  niche: string;
  growthPotential: number; // 0-100
  marketSaturation: number; // 0-100
  recommendedStrategy: string;
  pricePoint: string;
  justification: string;
  isAIRecommended?: boolean;
}

export interface MarketMetrics {
  month: string;
  demand: number;
  competition: number;
}
