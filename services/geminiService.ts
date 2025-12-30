
import { GoogleGenAI, Type } from "@google/genai";
import { ProductInsight } from "../types";

const apiKey = process.env.API_KEY;

if (!apiKey || apiKey === 'PLACEHOLDER_API_KEY') {
  console.warn('Warning: Gemini API key is not configured. Please set GEMINI_API_KEY in .env.local');
}

const ai = new GoogleGenAI({ apiKey: apiKey || '' });

export async function analyzeSocialAds(niche: string): Promise<ProductInsight[]> {
  if (!niche || niche.trim().length === 0) {
    throw new Error('Niche input is required');
  }

  if (!apiKey || apiKey === 'PLACEHOLDER_API_KEY') {
    throw new Error('Gemini API key is not configured. Please set GEMINI_API_KEY in .env.local');
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
      throw new Error('Invalid response from Gemini API');
    }

    const data = JSON.parse(response.text);

    if (!Array.isArray(data)) {
      throw new Error('Expected array response from Gemini API');
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
