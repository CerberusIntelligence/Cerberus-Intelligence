# Cerberus Protocol - Complete Setup Guide

This guide will walk you through setting up the complete Cerberus Protocol system with authentication, payment processing, and product intelligence features.

## 🚀 Quick Start (Development Mode)

For immediate testing without full backend setup:

```bash
# 1. Install dependencies
npm install

# 2. Create .env.local with Gemini API key only
echo "GEMINI_API_KEY=your_gemini_api_key" > .env.local

# 3. Run development server
npm run dev
```

Visit `http://localhost:3000` - The app will work in demo mode with simulated authentication.

---

## 📋 Full Production Setup

### Prerequisites

- Node.js 16+ installed
- Git installed
- A Google account (for Gemini AI)
- A Supabase account (free tier available)
- A Stripe account (for payments - test mode is free)

---

## Step 1: Clone and Install

```bash
git clone https://github.com/CerberusIntelligence/Cerberus-Intelligence.git
cd Cerberus-Intelligence
npm install
```

---

## Step 2: Get Your Gemini API Key

1. Visit https://ai.google.dev/
2. Click "Get API Key"
3. Create a new API key for your project
4. Copy the API key

---

## Step 3: Set Up Supabase

### Create Supabase Project

1. Go to https://supabase.com
2. Click "Start your project"
3. Create a new organization (free)
4. Create a new project:
   - Choose a name (e.g., "cerberus-protocol")
   - Set a strong database password (save this!)
   - Select a region close to you
   - Wait for project setup (~2 minutes)

### Get Supabase Credentials

1. In your project dashboard, click "Settings" (gear icon)
2. Go to "API" section
3. Copy these values:
   - **Project URL** → This is `VITE_SUPABASE_URL`
   - **anon/public** key → This is `VITE_SUPABASE_ANON_KEY`

### Create Database Tables

1. In Supabase dashboard, go to "SQL Editor"
2. Click "New Query"
3. Paste this SQL:

```sql
-- User Access Table
CREATE TABLE user_access (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE NOT NULL,
  payment_status TEXT CHECK (payment_status IN ('pending', 'completed', 'expired')) DEFAULT 'pending',
  stripe_payment_id TEXT,
  access_expires_at TIMESTAMP WITH TIME ZONE,
  amount_paid NUMERIC DEFAULT 350,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Enable Row Level Security
ALTER TABLE user_access ENABLE ROW LEVEL SECURITY;

-- Users can only view their own access data
CREATE POLICY "Users can view own access" ON user_access
  FOR SELECT USING (auth.uid() = user_id);

-- Users can insert their own access data
CREATE POLICY "Users can insert own access" ON user_access
  FOR INSERT WITH CHECK (auth.uid() = user_id);

-- Create index for faster lookups
CREATE INDEX idx_user_access_user_id ON user_access(user_id);
CREATE INDEX idx_user_access_expires ON user_access(access_expires_at);
```

4. Click "Run" to execute the SQL

### Enable Email Authentication

1. Go to "Authentication" → "Providers"
2. Make sure "Email" is enabled
3. Configure email templates (optional):
   - Go to "Authentication" → "Email Templates"
   - Customize confirmation and recovery emails

---

## Step 4: Set Up Stripe

### Create Stripe Account

1. Go to https://dashboard.stripe.com/register
2. Sign up for a free account
3. Activate your account (you'll need to verify your email)

### Get Stripe API Keys

1. In Stripe Dashboard, click "Developers" in top right
2. Go to "API keys" section
3. Toggle "Test mode" ON (top right)
4. Copy the **Publishable key** (starts with `pk_test_`)
   - This is your `VITE_STRIPE_PUBLISHABLE_KEY`

### Create Product in Stripe (Optional)

For production, you'll want to create a product:

1. Go to "Products" in Stripe Dashboard
2. Click "Add product"
3. Name: "Cerberus Protocol - 7 Day Access"
4. Price: $350 USD
5. Save the product

---

## Step 5: Configure Environment Variables

Create a `.env.local` file in the project root:

```env
# Google Gemini AI
GEMINI_API_KEY=your_gemini_api_key_here

# Supabase
VITE_SUPABASE_URL=https://your-project.supabase.co
VITE_SUPABASE_ANON_KEY=your_supabase_anon_key_here

# Stripe
VITE_STRIPE_PUBLISHABLE_KEY=pk_test_your_stripe_key_here

# Alibaba API (Optional - uses AI generation if not provided)
# VITE_ALIBABA_API_KEY=your_alibaba_key_here
```

**Important:** Never commit `.env.local` to git! It's already in `.gitignore`.

---

## Step 6: Run the Application

```bash
# Development mode
npm run dev

# Production build
npm run build
npm run preview
```

Visit `http://localhost:3000`

---

## 🧪 Testing the Application

### Test User Flow

1. **Visit Homepage** (`/`)
   - Click "System Access" button

2. **Sign Up** (`/login`)
   - Create account with email/password
   - Currently shows demo payment (Stripe integration next step)

3. **Access Dashboard** (`/dashboard`)
   - Automatically loads trending products
   - Search for niches (e.g., "Smart Home", "Wellness Tech")
   - Use filters (High Growth, Low Saturation)

4. **View Product Details** (click any product card)
   - See comprehensive validation data
   - Review ad analytics, Amazon competitors
   - Check sourcing information
   - Analyze profit projections

### Test Authentication

```bash
# Test user credentials (if using demo mode)
Email: test@cerberus.com
Password: test123456
```

---

## 🔧 Advanced Configuration

### Setting Up Stripe Webhooks (For Production)

To handle real payments, you need to set up webhooks:

1. **Create Webhook Endpoint:**
   - In Stripe Dashboard → Developers → Webhooks
   - Click "Add endpoint"
   - URL: `https://your-domain.com/api/webhooks/stripe`
   - Events to send: `payment_intent.succeeded`, `payment_intent.payment_failed`

2. **Create Supabase Edge Function:**
   ```bash
   # Install Supabase CLI
   npm install -g supabase

   # Create edge function
   supabase functions new stripe-webhook
   ```

3. **Implement webhook handler** (see `/supabase/functions/stripe-webhook/index.ts` in repo)

### Custom Domain Setup

1. Deploy to Vercel/Netlify/Railway
2. Configure custom domain
3. Update Supabase → Authentication → URL Configuration
4. Update Stripe webhook URL

---

## 📊 Data Flow Overview

```
User Signs Up
     ↓
Creates Supabase Auth User
     ↓
Redirects to Payment (Stripe)
     ↓
Payment Success → Webhook → Update user_access table
     ↓
User gains 7-day access
     ↓
Dashboard loads products via Gemini AI
     ↓
Product details show comprehensive validation data
```

---

## 🐛 Troubleshooting

### "Gemini API key not configured"
- Make sure `GEMINI_API_KEY` is in `.env.local`
- Restart the dev server after adding env variables

### "Supabase credentials not configured"
- Check `VITE_SUPABASE_URL` and `VITE_SUPABASE_ANON_KEY`
- Ensure they start with `VITE_` (required for Vite)

### "Network request failed" in Dashboard
- Check your internet connection
- Verify Gemini API key is valid
- Check browser console for detailed errors

### Products not loading
- Gemini API might be rate-limited (wait a moment and retry)
- Check if API key has quota remaining
- Try a different niche search term

### Login not working
- Verify Supabase project is active
- Check if email auth is enabled in Supabase
- Look for errors in browser console

---

## 💰 Cost Breakdown

### Free Tier Limits (Development)

- **Supabase:** 500MB database, 50,000 monthly active users
- **Gemini AI:** 60 requests per minute (free tier)
- **Stripe:** Unlimited test transactions in test mode

### Production Costs

- **Supabase Pro:** $25/month (recommended for production)
- **Gemini AI:** Pay as you go (~$0.10 per 1000 requests)
- **Stripe:** 2.9% + $0.30 per transaction
- **Hosting (Vercel/Netlify):** Free for small projects

---

## 🔐 Security Best Practices

1. **Never expose secret keys**
   - Keep `.env.local` out of git
   - Use `VITE_` prefix only for public keys

2. **Enable RLS (Row Level Security)**
   - Already configured in the SQL above
   - Users can only access their own data

3. **Use HTTPS in production**
   - Vercel/Netlify provide free SSL
   - Required for Stripe payments

4. **Implement rate limiting**
   - Consider adding rate limits to API calls
   - Prevent abuse of Gemini API

---

## 📚 Additional Resources

- [Supabase Documentation](https://supabase.com/docs)
- [Stripe Documentation](https://stripe.com/docs)
- [Gemini AI Documentation](https://ai.google.dev/docs)
- [React Router Documentation](https://reactrouter.com)
- [Vite Documentation](https://vitejs.dev)

---

## 🆘 Getting Help

- **GitHub Issues:** https://github.com/CerberusIntelligence/Cerberus-Intelligence/issues
- **Documentation:** See README.md and QUICKSTART.md

---

## ✅ Verification Checklist

- [ ] Dependencies installed (`npm install`)
- [ ] Gemini API key obtained and added to `.env.local`
- [ ] Supabase project created
- [ ] Database tables created
- [ ] Supabase credentials added to `.env.local`
- [ ] Stripe account created (test mode)
- [ ] Stripe publishable key added to `.env.local`
- [ ] Dev server runs without errors (`npm run dev`)
- [ ] Can sign up and login
- [ ] Dashboard loads products
- [ ] Product detail page shows full data

Once all items are checked, you're ready for production deployment!
