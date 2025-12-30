import { GoogleGenAI, Type } from "@google/genai";
import { ProductInsight, DetailedProduct } from "../types";
import { getAlibabaSourceInfo } from "./alibabaService";

const apiKey = process.env.API_KEY;

if (!apiKey || apiKey === 'PLACEHOLDER_API_KEY') {
  console.warn('⚠️  Warning: Gemini API key is not configured. Please set GEMINI_API_KEY in .env.local');
}

const ai = new GoogleGenAI({ apiKey: apiKey || '' });

export async function analyzeSocialAds(niche: string): Promise<ProductInsight[]> {
  // Validate input
  if (!niche || niche.trim().length === 0) {
    throw new Error('Niche input is required. Please enter a product category or market to analyze.');
  }

  // Validate API key
  if (!apiKey || apiKey === 'PLACEHOLDER_API_KEY') {
    throw new Error('Gemini API key is not configured. Please set GEMINI_API_KEY in .env.local and restart the development server.');
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

  // Validate API key
  if (!apiKey || apiKey === 'PLACEHOLDER_API_KEY') {
    throw new Error('Gemini API key is not configured. Please set GEMINI_API_KEY in .env.local and restart the development server.');
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
