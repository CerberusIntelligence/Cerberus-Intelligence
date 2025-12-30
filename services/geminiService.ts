import { GoogleGenAI, Type } from "@google/genai";
import { ProductInsight, DetailedProduct } from "../types";
import { getAlibabaSourceInfo } from "./alibabaService";

const apiKey = process.env.API_KEY;
const isGeminiConfigured = apiKey && apiKey !== 'PLACEHOLDER_API_KEY';

if (!isGeminiConfigured) {
  console.warn('⚠️  Gemini API key not configured. Running in DEMO mode with mock data.');
  console.warn('💡  To use real AI analysis, get a free API key from https://ai.google.dev/');
}

const ai = isGeminiConfigured ? new GoogleGenAI({ apiKey: apiKey || '' }) : null;

// Export the configuration status
export const isGeminiInDemoMode = !isGeminiConfigured;

/**
 * Generate realistic mock data for demo mode
 */
function generateMockProductInsights(niche: string): ProductInsight[] {
  const productTemplates = [
    { name: "Smart LED Strip Lights", adCount: 23, growth: 85, saturation: 35 },
    { name: "Portable Neck Fan", adCount: 18, growth: 78, saturation: 42 },
    { name: "Magnetic Phone Mount", adCount: 31, growth: 72, saturation: 48 },
    { name: "Wireless Charging Station", adCount: 27, growth: 81, saturation: 38 }
  ];

  return productTemplates.map((template, index) => ({
    name: `${niche} - ${template.name}`,
    niche: niche,
    growthPotential: template.growth,
    marketSaturation: template.saturation,
    recommendedStrategy: `Target ${niche} enthusiasts via TikTok/Instagram with UGC content. Focus on problem-solution angle.`,
    pricePoint: `$${(19.99 + index * 5).toFixed(2)} - $${(34.99 + index * 5).toFixed(2)}`,
    justification: `Low saturation (${template.saturation}%), ${template.adCount} active ads detected, strong engagement metrics on social platforms.`,
    adCount: template.adCount,
    isAIRecommended: true
  }));
}

/**
 * Generate comprehensive mock data for detailed product analysis
 */
async function generateMockDetailedProducts(niche: string): Promise<DetailedProduct[]> {
  const mockProducts = [
    {
      name: `Premium ${niche} Smart Device`,
      basePrice: 29.99,
      growth: 87,
      saturation: 32,
      adCount: 24,
      searchVolume: 45000
    },
    {
      name: `Portable ${niche} Kit`,
      basePrice: 24.99,
      growth: 82,
      saturation: 38,
      adCount: 19,
      searchVolume: 38000
    },
    {
      name: `${niche} Pro Edition`,
      basePrice: 39.99,
      growth: 79,
      saturation: 41,
      adCount: 28,
      searchVolume: 52000
    },
    {
      name: `Wireless ${niche} Station`,
      basePrice: 34.99,
      growth: 85,
      saturation: 35,
      adCount: 22,
      searchVolume: 41000
    },
    {
      name: `Mini ${niche} Accessory`,
      basePrice: 19.99,
      growth: 91,
      saturation: 28,
      adCount: 16,
      searchVolume: 36000
    },
    {
      name: `Magnetic ${niche} Holder`,
      basePrice: 22.99,
      growth: 88,
      saturation: 30,
      adCount: 20,
      searchVolume: 43000
    },
    {
      name: `LED ${niche} Light`,
      basePrice: 27.99,
      growth: 83,
      saturation: 37,
      adCount: 25,
      searchVolume: 47000
    },
    {
      name: `Adjustable ${niche} Stand`,
      basePrice: 32.99,
      growth: 80,
      saturation: 39,
      adCount: 21,
      searchVolume: 39000
    }
  ];

  const products = await Promise.all(
    mockProducts.map(async (template, index) => {
      const alibabaData = await getAlibabaSourceInfo(template.name);
      const unitCost = template.basePrice * 0.35; // 35% COGS
      const profitMargin = ((template.basePrice - unitCost) / template.basePrice * 100);

      return {
        id: `${niche.toLowerCase().replace(/\s+/g, '-')}-${index}-${Date.now()}`,
        name: template.name,
        niche: niche,
        growthPotential: template.growth,
        marketSaturation: template.saturation,
        recommendedStrategy: `Launch with TikTok creators, scale to Facebook once proof-of-concept validated. Target ${niche} enthusiasts.`,
        pricePoint: `$${template.basePrice.toFixed(2)}`,
        justification: `${template.adCount} active ads across platforms, ${template.growth}% growth potential, low saturation at ${template.saturation}%.`,
        isAIRecommended: true,

        adAnalytics: {
          totalAds: template.adCount,
          platforms: [
            { name: "TikTok", count: Math.floor(template.adCount * 0.45), engagement: 78 + index * 2 },
            { name: "Facebook", count: Math.floor(template.adCount * 0.30), engagement: 65 + index * 2 },
            { name: "Instagram", count: Math.floor(template.adCount * 0.20), engagement: 72 + index * 2 },
            { name: "YouTube", count: Math.floor(template.adCount * 0.05), engagement: 81 + index * 2 }
          ],
          topPerformingAds: [
            { platform: "TikTok", engagement: 89, link: "#demo-ad-1" },
            { platform: "Facebook", engagement: 84, link: "#demo-ad-2" },
            { platform: "Instagram", engagement: 86, link: "#demo-ad-3" }
          ]
        },

        amazonData: {
          competitorProducts: [
            {
              title: `${template.name} - Best Seller`,
              price: template.basePrice + 5,
              rating: 4.3 + (index % 3) * 0.2,
              reviews: 2847 + index * 300,
              link: "#demo-amazon-product",
              asin: `B0${index}XYZ${Math.floor(Math.random() * 1000)}`
            },
            {
              title: `Premium ${template.name}`,
              price: template.basePrice + 10,
              rating: 4.5 + (index % 2) * 0.1,
              reviews: 1923 + index * 200,
              link: "#demo-amazon-product",
              asin: `B0${index}ABC${Math.floor(Math.random() * 1000)}`
            },
            {
              title: `Budget ${template.name}`,
              price: template.basePrice - 5,
              rating: 3.9 + (index % 4) * 0.1,
              reviews: 1456 + index * 150,
              link: "#demo-amazon-product",
              asin: `B0${index}DEF${Math.floor(Math.random() * 1000)}`
            }
          ],
          estimatedRevenue: 125000 + index * 15000,
          avgPrice: template.basePrice
        },

        competitors: {
          websites: [
            {
              url: `www.${niche.toLowerCase().replace(/\s+/g, '')}-store.com`,
              monthlyVisitors: 45000 + index * 5000,
              estimatedRevenue: 89000 + index * 8000
            },
            {
              url: `shop${niche.toLowerCase().replace(/\s+/g, '')}.com`,
              monthlyVisitors: 32000 + index * 4000,
              estimatedRevenue: 67000 + index * 6000
            }
          ],
          marketShare: 12 + index * 2
        },

        alibabaLink: alibabaData.alibabaLink,
        sourcing: {
          moq: alibabaData.moq,
          unitPrice: alibabaData.unitPrice,
          shippingTime: alibabaData.shippingTime,
          supplier: alibabaData.supplier
        },

        metrics: {
          searchVolume: template.searchVolume,
          trendDirection: index % 3 === 0 ? 'up' : index % 3 === 1 ? 'stable' : 'up',
          seasonality: index % 2 === 0 ? 'Year-round demand' : 'Peak: Q4',
          profitMargin: Math.round(profitMargin),
          breakEvenUnits: Math.ceil(3500 / (template.basePrice - unitCost))
        }
      } as DetailedProduct;
    })
  );

  return products;
}

export async function analyzeSocialAds(niche: string): Promise<ProductInsight[]> {
  // Validate input
  if (!niche || niche.trim().length === 0) {
    throw new Error('Niche input is required. Please enter a product category or market to analyze.');
  }

  // If in demo mode, return mock data
  if (!isGeminiConfigured) {
    console.log('🎭 Demo mode: Returning mock product insights');
    // Simulate API delay for realism
    await new Promise(resolve => setTimeout(resolve, 800));
    return generateMockProductInsights(niche);
  }

  try {
    const response = await ai.models.generateContent({
      model: "gemini-3-flash-preview",
      contents: `Act as Cerberus AI, an elite e-commerce product validator.
      Analyze the current social media landscape (TikTok, Meta, IG) for the niche: "${niche}".
      Identify 4 winning products that meet these STRICT criteria:
      1. Low Saturation: Still in early growth phase.
      2. Ad Threshold: Currently between 5 and 50 active ads (detecting "under-the-radar" scaling).
      3. Scalability: Proven engagement but not yet mass-marketed.

      For each, provide: name, niche, growth potential (0-100), market saturation (0-100), price point, justification (why Cerberus flagged it), and a specific scaling strategy.`,
      config: {
        responseMimeType: "application/json",
        responseSchema: {
          type: Type.ARRAY,
          items: {
            type: Type.OBJECT,
            properties: {
              name: { type: Type.STRING },
              niche: { type: Type.STRING },
              growthPotential: { type: Type.NUMBER },
              marketSaturation: { type: Type.NUMBER },
              recommendedStrategy: { type: Type.STRING },
              pricePoint: { type: Type.STRING },
              justification: { type: Type.STRING },
              adCount: { type: Type.NUMBER, description: "Active ads found across platforms" },
              isAIRecommended: { type: Type.BOOLEAN }
            },
            required: ["name", "niche", "growthPotential", "marketSaturation", "recommendedStrategy", "pricePoint", "justification", "adCount"]
          }
        }
      }
    });

    if (!response || !response.text) {
      throw new Error('Invalid response from Gemini API. Please try again.');
    }

    const data = JSON.parse(response.text);

    if (!Array.isArray(data)) {
      throw new Error('Unexpected response format from Gemini API. Expected an array of products.');
    }

    if (data.length === 0) {
      throw new Error('No products found for this niche. Try a different search term.');
    }

    return data;
  } catch (error) {
    if (error instanceof Error) {
      console.error("Cerberus Protocol Error:", error.message);
      throw new Error(`Failed to analyze niche: ${error.message}`);
    }
    console.error("Cerberus Protocol Error:", error);
    throw new Error('Failed to analyze niche: Unknown error occurred');
  }
}

/**
 * Get detailed product intelligence data with comprehensive validation metrics
 * This provides everything needed for product validation and decision making
 */
export async function getDetailedProductData(niche: string): Promise<DetailedProduct[]> {
  // Validate input
  if (!niche || niche.trim().length === 0) {
    throw new Error('Niche input is required. Please enter a product category or market to analyze.');
  }

  // If in demo mode, return mock data
  if (!isGeminiConfigured) {
    console.log('🎭 Demo mode: Returning comprehensive mock product data');
    // Simulate API delay for realism
    await new Promise(resolve => setTimeout(resolve, 1200));
    return generateMockDetailedProducts(niche);
  }

  try {
    const response = await ai.models.generateContent({
      model: "gemini-3-flash-preview",
      contents: `Act as Cerberus AI, an elite e-commerce product intelligence system.

Analyze the niche: "${niche}" and provide 8 HIGH-POTENTIAL products with COMPREHENSIVE validation data.

For each product, provide detailed intelligence across ALL categories:

1. BASIC INFO:
   - Product name (specific, searchable)
   - Niche category
   - Growth potential (0-100)
   - Market saturation (0-100)
   - Recommended price point
   - Scaling strategy
   - Justification for selection

2. AD ANALYTICS:
   - Total active ads across all platforms
   - Platform breakdown (TikTok, Facebook, Instagram, YouTube) with counts and engagement rates
   - Top 3 performing ads with platform, engagement score (0-100), and example ad angles

3. AMAZON MARKETPLACE DATA:
   - 3-5 competitor products with: title, price, rating (1-5), review count, ASIN
   - Estimated total market revenue
   - Average selling price

4. COMPETITOR ANALYSIS:
   - 2-3 competitor websites with: URL, monthly visitors, estimated monthly revenue
   - Your estimated market share opportunity (%)

5. ADVANCED METRICS:
   - Monthly search volume
   - Trend direction (up/down/stable)
   - Seasonality pattern
   - Estimated profit margin (%)
   - Break-even units needed

Ensure data is realistic and based on current e-commerce trends. Make numbers believable and internally consistent.`,
      config: {
        responseMimeType: "application/json",
        responseSchema: {
          type: Type.ARRAY,
          items: {
            type: Type.OBJECT,
            properties: {
              name: { type: Type.STRING },
              niche: { type: Type.STRING },
              growthPotential: { type: Type.NUMBER },
              marketSaturation: { type: Type.NUMBER },
              recommendedStrategy: { type: Type.STRING },
              pricePoint: { type: Type.STRING },
              justification: { type: Type.STRING },
              isAIRecommended: { type: Type.BOOLEAN },

              adAnalytics: {
                type: Type.OBJECT,
                properties: {
                  totalAds: { type: Type.NUMBER },
                  platforms: {
                    type: Type.ARRAY,
                    items: {
                      type: Type.OBJECT,
                      properties: {
                        name: { type: Type.STRING },
                        count: { type: Type.NUMBER },
                        engagement: { type: Type.NUMBER }
                      },
                      required: ["name", "count", "engagement"]
                    }
                  },
                  topPerformingAds: {
                    type: Type.ARRAY,
                    items: {
                      type: Type.OBJECT,
                      properties: {
                        platform: { type: Type.STRING },
                        engagement: { type: Type.NUMBER },
                        link: { type: Type.STRING }
                      },
                      required: ["platform", "engagement", "link"]
                    }
                  }
                },
                required: ["totalAds", "platforms", "topPerformingAds"]
              },

              amazonData: {
                type: Type.OBJECT,
                properties: {
                  competitorProducts: {
                    type: Type.ARRAY,
                    items: {
                      type: Type.OBJECT,
                      properties: {
                        title: { type: Type.STRING },
                        price: { type: Type.NUMBER },
                        rating: { type: Type.NUMBER },
                        reviews: { type: Type.NUMBER },
                        link: { type: Type.STRING },
                        asin: { type: Type.STRING }
                      },
                      required: ["title", "price", "rating", "reviews", "link"]
                    }
                  },
                  estimatedRevenue: { type: Type.NUMBER },
                  avgPrice: { type: Type.NUMBER }
                },
                required: ["competitorProducts", "estimatedRevenue", "avgPrice"]
              },

              competitors: {
                type: Type.OBJECT,
                properties: {
                  websites: {
                    type: Type.ARRAY,
                    items: {
                      type: Type.OBJECT,
                      properties: {
                        url: { type: Type.STRING },
                        monthlyVisitors: { type: Type.NUMBER },
                        estimatedRevenue: { type: Type.NUMBER }
                      },
                      required: ["url", "monthlyVisitors", "estimatedRevenue"]
                    }
                  },
                  marketShare: { type: Type.NUMBER }
                },
                required: ["websites", "marketShare"]
              },

              metrics: {
                type: Type.OBJECT,
                properties: {
                  searchVolume: { type: Type.NUMBER },
                  trendDirection: { type: Type.STRING },
                  seasonality: { type: Type.STRING },
                  profitMargin: { type: Type.NUMBER },
                  breakEvenUnits: { type: Type.NUMBER }
                },
                required: ["searchVolume", "trendDirection", "seasonality", "profitMargin", "breakEvenUnits"]
              }
            },
            required: ["name", "niche", "growthPotential", "marketSaturation", "recommendedStrategy", "pricePoint", "justification", "adAnalytics", "amazonData", "competitors", "metrics"]
          }
        }
      }
    });

    if (!response || !response.text) {
      throw new Error('Invalid response from Gemini API. Please try again.');
    }

    const data = JSON.parse(response.text);

    if (!Array.isArray(data)) {
      throw new Error('Unexpected response format from Gemini API. Expected an array of products.');
    }

    if (data.length === 0) {
      throw new Error('No products found for this niche. Try a different search term.');
    }

    // Enhance each product with Alibaba sourcing data and generate IDs
    const enhancedProducts = await Promise.all(
      data.map(async (product, index) => {
        // Get Alibaba sourcing info
        const alibabaData = await getAlibabaSourceInfo(product.name);

        return {
          ...product,
          id: `${niche.toLowerCase().replace(/\s+/g, '-')}-${index}-${Date.now()}`,
          alibabaLink: alibabaData.alibabaLink,
          sourcing: {
            moq: alibabaData.moq,
            unitPrice: alibabaData.unitPrice,
            shippingTime: alibabaData.shippingTime,
            supplier: alibabaData.supplier
          }
        } as DetailedProduct;
      })
    );

    return enhancedProducts;
  } catch (error) {
    if (error instanceof Error) {
      console.error("Cerberus Protocol Error:", error.message);
      throw new Error(`Failed to analyze niche: ${error.message}`);
    }
    console.error("Cerberus Protocol Error:", error);
    throw new Error('Failed to analyze niche: Unknown error occurred');
  }
}
