import { loadStripe, Stripe } from '@stripe/stripe-js';

const stripePublishableKey = import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY || '';

if (!stripePublishableKey) {
  console.warn('⚠️  Stripe publishable key not configured. Please set VITE_STRIPE_PUBLISHABLE_KEY in .env.local');
}

let stripePromise: Promise<Stripe | null>;

export const getStripe = () => {
  if (!stripePromise) {
    stripePromise = loadStripe(stripePublishableKey);
  }
  return stripePromise;
};

export const stripeHelpers = {
  /**
   * Create a Stripe Payment Intent for Cerberus Protocol Access
   * Note: This is a client-side implementation. In production, you should
   * create the payment intent on your backend/Supabase Edge Function
   */
  async createPaymentIntent(userId: string, email: string) {
    // In production, call your backend API/Supabase Edge Function
    // For now, we'll return a mock structure for development

    // TODO: Replace with actual backend call:
    // const response = await fetch('/api/create-payment-intent', {
    //   method: 'POST',
    //   headers: { 'Content-Type': 'application/json' },
    //   body: JSON.stringify({ userId, email, amount: 35000 }) // $350 in cents
    // });
    // return response.json();

    console.log('Payment intent requested for:', { userId, email });

    return {
      clientSecret: 'mock_client_secret', // This will be replaced with real Stripe secret
      paymentIntentId: 'mock_pi_' + Date.now(),
    };
  },

  /**
   * Verify payment completion
   * This should be called after Stripe payment succeeds
   */
  async verifyPayment(paymentIntentId: string): Promise<boolean> {
    // In production, verify with your backend
    // For development, return true
    console.log('Verifying payment:', paymentIntentId);
    return true;
  },

  /**
   * Format price for display
   */
  formatPrice(amount: number, currency: string = 'USD'): string {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency,
    }).format(amount);
  }
};

// Stripe configuration constants
export const STRIPE_CONFIG = {
  PRODUCT_NAME: 'Cerberus Protocol Access - 7 Days',
  AMOUNT: 350, // $350
  CURRENCY: 'usd',
  ACCESS_DURATION_DAYS: 7,
};
