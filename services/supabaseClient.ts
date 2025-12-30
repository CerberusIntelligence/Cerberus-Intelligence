import { createClient } from '@supabase/supabase-js';
import { User, UserAccess } from '../types';

const supabaseUrl = import.meta.env.VITE_SUPABASE_URL || 'https://placeholder.supabase.co';
const supabaseAnonKey = import.meta.env.VITE_SUPABASE_ANON_KEY || 'placeholder-key';

const isConfigured = import.meta.env.VITE_SUPABASE_URL && import.meta.env.VITE_SUPABASE_ANON_KEY;

if (!isConfigured) {
  console.warn('⚠️  Supabase credentials not configured. Running in DEMO mode. Please set VITE_SUPABASE_URL and VITE_SUPABASE_ANON_KEY in .env.local for full functionality.');
}

export const supabase = createClient(supabaseUrl, supabaseAnonKey, {
  auth: {
    autoRefreshToken: true,
    persistSession: true,
    detectSessionInUrl: true
  }
});

// Export flag to check if Supabase is configured
export const isSupabaseConfigured = isConfigured;

// Auth Helpers
export const authHelpers = {
  async signUp(email: string, password: string) {
    const { data, error } = await supabase.auth.signUp({
      email,
      password,
    });

    if (error) throw error;
    return data;
  },

  async signIn(email: string, password: string) {
    const { data, error } = await supabase.auth.signInWithPassword({
      email,
      password,
    });

    if (error) throw error;
    return data;
  },

  async signOut() {
    const { error } = await supabase.auth.signOut();
    if (error) throw error;
  },

  async getCurrentUser() {
    const { data: { user }, error } = await supabase.auth.getUser();
    if (error) throw error;
    return user;
  },

  async getSession() {
    const { data: { session }, error } = await supabase.auth.getSession();
    if (error) throw error;
    return session;
  }
};

// User Access Helpers
export const accessHelpers = {
  async getUserAccess(userId: string): Promise<UserAccess | null> {
    const { data, error } = await supabase
      .from('user_access')
      .select('*')
      .eq('user_id', userId)
      .order('created_at', { ascending: false })
      .limit(1)
      .single();

    if (error && error.code !== 'PGRST116') {
      // PGRST116 = no rows returned
      console.error('Error fetching user access:', error);
      return null;
    }

    return data as UserAccess | null;
  },

  async createUserAccess(userId: string, stripePaymentId: string): Promise<UserAccess> {
    const expiresAt = new Date();
    expiresAt.setDate(expiresAt.getDate() + 7); // 7 days from now

    const { data, error } = await supabase
      .from('user_access')
      .insert([
        {
          user_id: userId,
          payment_status: 'completed',
          stripe_payment_id: stripePaymentId,
          access_expires_at: expiresAt.toISOString(),
          amount_paid: 350,
        },
      ])
      .select()
      .single();

    if (error) throw error;
    return data as UserAccess;
  },

  async updatePaymentStatus(
    userId: string,
    paymentId: string,
    status: 'completed' | 'pending' | 'expired'
  ): Promise<void> {
    const { error } = await supabase
      .from('user_access')
      .update({ payment_status: status, stripe_payment_id: paymentId })
      .eq('user_id', userId);

    if (error) throw error;
  },

  hasActiveAccess(userAccess: UserAccess | null): boolean {
    if (!userAccess) return false;
    if (userAccess.payment_status !== 'completed') return false;

    const expiresAt = new Date(userAccess.access_expires_at);
    const now = new Date();

    return expiresAt > now;
  },

  getDaysRemaining(userAccess: UserAccess | null): number {
    if (!userAccess) return 0;

    const expiresAt = new Date(userAccess.access_expires_at);
    const now = new Date();
    const diffTime = expiresAt.getTime() - now.getTime();
    const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

    return Math.max(0, diffDays);
  }
};
