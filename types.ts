// Base Product Insight (from landing page analysis)
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

// Detailed Product with full intelligence data
export interface DetailedProduct extends ProductInsight {
  id: string;

  // Ad Analytics
  adAnalytics: {
    totalAds: number;
    platforms: Array<{
      name: string;
      count: number;
      engagement: number;
    }>;
    topPerformingAds: Array<{
      platform: string;
      engagement: number;
      link: string;
      thumbnail?: string;
    }>;
  };

  // Amazon Data
  amazonData: {
    competitorProducts: Array<{
      title: string;
      price: number;
      rating: number;
      reviews: number;
      link: string;
      asin?: string;
    }>;
    estimatedRevenue: number;
    avgPrice: number;
  };

  // Competitor Analysis
  competitors: {
    websites: Array<{
      url: string;
      monthlyVisitors: number;
      estimatedRevenue: number;
    }>;
    marketShare: number;
  };

  // Sourcing
  alibabaLink: string;
  sourcing: {
    moq: number;
    unitPrice: number;
    shippingTime: string;
    supplier: string;
  };

  // Advanced Metrics
  metrics: {
    searchVolume: number;
    trendDirection: 'up' | 'down' | 'stable';
    seasonality: string;
    profitMargin: number;
    breakEvenUnits: number;
  };
}

// Market Metrics for Charts
export interface MarketMetrics {
  month: string;
  demand: number;
  competition: number;
}

// User Authentication & Access
export interface User {
  id: string;
  email: string;
  created_at: string;
}

export interface UserAccess {
  id: string;
  user_id: string;
  payment_status: 'pending' | 'completed' | 'expired';
  stripe_payment_id: string | null;
  access_expires_at: string;
  amount_paid: number;
  created_at: string;
  updated_at: string;
}

// Auth Context Types
export interface AuthContextType {
  user: User | null;
  userAccess: UserAccess | null;
  isLoading: boolean;
  signIn: (email: string, password: string) => Promise<void>;
  signUp: (email: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
  checkAccess: () => Promise<boolean>;
  hasActiveAccess: boolean;
}

// Payment Types
export interface PaymentIntent {
  amount: number;
  currency: string;
  description: string;
}
