<div align="center">
<img width="1200" height="475" alt="GHBanner" src="https://github.com/user-attachments/assets/0aa67016-6eaf-458a-adb2-6e31a0763ed6" />
</div>

# Cerberus Protocol

**A high-end AI-powered e-commerce product validation and scaling tool**

Inspired by the mythological gatekeeper, Cerberus Protocol guards your investments by finding and validating multi-million dollar product opportunities with precision. The tool intercepts emerging growth vectors from billions of data points daily across social media platforms.

## Features

- **AI-Powered Product Analysis**: Uses Google's Gemini AI to analyze social media trends
- **Real-Time Market Intelligence**: Identifies products in early growth phase with low saturation
- **Smart Filtering**: Detects "under-the-radar" scaling opportunities (5-50 active ads)
- **Interactive UI**: Premium dark-themed interface with proximity-based interactions
- **Data Visualization**: Real-time charts showing demand and competition metrics

## Tech Stack

- **Frontend**: React 19.2+ with TypeScript
- **Build Tool**: Vite 6.2+
- **AI**: Google Gemini AI (gemini-3-flash-preview)
- **UI Libraries**:
  - Lucide React (icons)
  - Recharts (data visualization)
  - Tailwind CSS (styling)

## Prerequisites

- Node.js 16+
- A valid Google Gemini API key ([Get one here](https://ai.google.dev/))

## Installation

1. **Clone the repository**
   ```bash
   git clone <your-repo-url>
   cd cerberus-protocol
   ```

2. **Install dependencies**
   ```bash
   npm install
   ```

3. **Configure environment variables**

   Copy `.env.example` to `.env.local`:
   ```bash
   cp .env.example .env.local
   ```

   Then edit `.env.local` and add your Gemini API key:
   ```
   GEMINI_API_KEY=your_actual_api_key_here
   ```

4. **Run the development server**
   ```bash
   npm run dev
   ```

5. **Open your browser**

   Navigate to `http://localhost:3000`

## Available Scripts

- `npm run dev` - Start development server on port 3000
- `npm run build` - Build production bundle
- `npm run preview` - Preview production build locally

## Usage

1. Enter a product niche in the search field (e.g., "Luxury Wellness Tech")
2. Click "EXECUTE PROTOCOL" to analyze the market
3. View AI-generated product recommendations with:
   - Growth potential scores
   - Market saturation levels
   - Recommended pricing
   - Scaling strategies
   - Active ad counts

## Project Structure

```
cerberus-protocol/
├── components/           # React components
│   ├── AnimatedBackground.tsx
│   ├── CerberusLogo.tsx
│   ├── InteractiveButton.tsx
│   ├── PremiumCard.tsx
│   └── ProximityWrapper.tsx
├── services/            # API services
│   └── geminiService.ts
├── App.tsx              # Main application component
├── index.tsx            # Application entry point
├── types.ts             # TypeScript type definitions
├── vite.config.ts       # Vite configuration
├── tsconfig.json        # TypeScript configuration
└── package.json         # Dependencies and scripts
```

## Security Notes

- **Never commit your `.env.local` file** - it contains sensitive API keys
- The `.gitignore` file is configured to exclude environment files
- Keep your Gemini API key secure and rotate it if exposed

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is private and proprietary.

## Support

For issues or questions, please open an issue on GitHub.
