# Quick Start Guide

## Push to GitHub (Choose One Method)

### Method 1: Run the Deploy Script (Easiest)
```bash
# On Windows:
deploy.bat

# On Linux/Mac:
./deploy.sh
```

### Method 2: Manual GitHub Web + Git
1. Go to https://github.com/new
2. Name: `cerberus-protocol`
3. Click "Create repository" (don't initialize anything)
4. Run:
```bash
git remote add origin https://github.com/YOUR_USERNAME/cerberus-protocol.git
git branch -M main
git push -u origin main
```

## After Pushing to GitHub

### Set Up Your API Key
1. Get a Gemini API key from https://ai.google.dev/
2. Copy `.env.example` to `.env.local`:
   ```bash
   cp .env.example .env.local
   ```
3. Edit `.env.local` and add your API key:
   ```
   GEMINI_API_KEY=your_actual_api_key_here
   ```

### Run the Application
```bash
npm install  # Already done
npm run dev
```

Open http://localhost:3000 in your browser.

## What's Been Improved

✅ **Enhanced Error Handling**
  - API key validation
  - Comprehensive error messages
  - Input validation

✅ **Better Documentation**
  - Detailed README with installation steps
  - Project structure overview
  - Security notes

✅ **Security**
  - Enhanced .gitignore
  - .env.example template
  - Environment variable validation

✅ **Code Quality**
  - TypeScript strict mode
  - Proper error handling in all services
  - Clean component structure

## File Structure

```
cerberus-protocol/
├── components/              # UI components
│   ├── AnimatedBackground.tsx
│   ├── CerberusLogo.tsx
│   ├── InteractiveButton.tsx
│   ├── PremiumCard.tsx
│   └── ProximityWrapper.tsx
├── services/
│   └── geminiService.ts    # AI integration (enhanced)
├── App.tsx                 # Main app (enhanced error handling)
├── .env.example            # NEW: Environment template
├── .gitignore              # ENHANCED: Better security
├── README.md               # ENHANCED: Full documentation
├── DEPLOY.md               # NEW: Deployment guide
├── deploy.bat              # NEW: Windows deploy script
└── deploy.sh               # NEW: Linux/Mac deploy script
```

## Next Steps

1. **Push to GitHub** (see above)
2. **Configure API key** (see above)
3. **Test the application** locally
4. **Deploy to production** (optional)
   - Vercel: `vercel`
   - Netlify: `netlify deploy`
   - GitHub Pages: Configure in repo settings

## Need Help?

Check these files:
- `README.md` - Full documentation
- `DEPLOY.md` - Detailed deployment instructions
- `.env.example` - Environment configuration template
