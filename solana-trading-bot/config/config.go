package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// Mode
	PaperTrading       bool    // Enable paper trading mode (no real trades)

	// Telegram Settings
	TelegramAPIID      int
	TelegramAPIHash    string
	TelegramBotToken   string
	TelegramChatID     int64
	TelegramPhone      string   // Phone number for MTProto auth
	MonitoredChannels  []string

	// Solana Settings
	SolanaRPCURL        string
	SolanaWSURL         string
	AdditionalRPCURLs   []string // Extra low-latency RPC endpoints
	PrivateKey          string

	// Trading Settings
	PortfolioSize      float64 // Total portfolio in SOL
	RiskPerTrade       float64 // Percentage risk per trade (0.02 = 2%)
	MaxPositionSize    float64 // Max SOL per trade
	MinLiquidity       float64 // Minimum liquidity in USD
	SlippageBPS        int     // Slippage in basis points
	PriorityFeeLamports uint64 // Priority fee for faster execution

	// Ultra-low-latency risk guards
	MaxSlippagePercent    float64 // Abort if projected slippage > this (%)
	MaxLpSharePercent     float64 // Abort if position > this % of LP
	MaxLpDropPercent      float64 // Abort if LP dropped > this % over window
	LpDropWindowSeconds   int     // Window for LP drop check

	// Safety Thresholds
	MinLiquidityLocked    float64 // Minimum % of LP locked
	MaxTopHolderPercent   float64 // Max % single holder can own
	MaxDevWalletPercent   float64 // Max % dev wallet can hold
	MinHolderCount        int     // Minimum number of holders
	MaxMintAuthority      bool    // Reject if mint authority exists
	MaxFreezeAuthority    bool    // Reject if freeze authority exists

	// Take Profit / Stop Loss
	TakeProfitLevels      []float64 // e.g., [2.0, 5.0, 10.0] for 2x, 5x, 10x
	TakeProfitPercents    []float64 // e.g., [0.3, 0.3, 0.4] sell 30%, 30%, 40%
	StopLossPercent       float64   // e.g., 0.5 = sell if down 50%
	TrailingStopPercent   float64   // e.g., 0.2 = 20% trailing stop
	TimeoutMinutes        int       // Exit if no pump after X minutes

	// Wallet Tracking
	TrackedWallets        []string  // Wallets to copy trade
	WalletTrackingEnabled bool

	// Twitter/Social
	TwitterBearerToken    string
	TwitterEnabled        bool
	TwitterKeywords       []string  // Additional keywords to track

	// API Keys
	BirdeyeAPIKey         string
	HeliusAPIKey          string

	// Rate Limiting
	MaxTradesPerHour      int
	CooldownSeconds       int

	// Signal freshness — skip signals older than this many minutes
	SignalMaxAgeMinutes   int

	// Flow / distribution-based exits
	DistributionThreshold float64 // Net sell flow threshold (0-1) to trigger exit
}

func Load() (*Config, error) {
	godotenv.Load()

	apiID, _ := strconv.Atoi(getEnv("TELEGRAM_API_ID", "0"))
	chatID, _ := strconv.ParseInt(getEnv("TELEGRAM_CHAT_ID", "0"), 10, 64)
	portfolioSize, _ := strconv.ParseFloat(getEnv("PORTFOLIO_SIZE_SOL", "3.5"), 64)
	riskPerTrade, _ := strconv.ParseFloat(getEnv("RISK_PER_TRADE", "0.02"), 64)
	maxPosition, _ := strconv.ParseFloat(getEnv("MAX_POSITION_SOL", "0.1"), 64)
	minLiquidity, _ := strconv.ParseFloat(getEnv("MIN_LIQUIDITY_USD", "10000"), 64)
	slippage, _ := strconv.Atoi(getEnv("SLIPPAGE_BPS", "500"))
	priorityFee, _ := strconv.ParseUint(getEnv("PRIORITY_FEE_LAMPORTS", "100000"), 10, 64)

	maxSlipPct, _ := strconv.ParseFloat(getEnv("MAX_SLIPPAGE_PERCENT", "3.0"), 64)
	maxLpSharePct, _ := strconv.ParseFloat(getEnv("MAX_LP_SHARE_PERCENT", "1.5"), 64)
	maxLpDropPct, _ := strconv.ParseFloat(getEnv("MAX_LP_DROP_PERCENT", "30.0"), 64)
	lpDropWindowSecs, _ := strconv.Atoi(getEnv("LP_DROP_WINDOW_SECONDS", "60"))

	minLPLocked, _ := strconv.ParseFloat(getEnv("MIN_LP_LOCKED_PERCENT", "80"), 64)
	maxTopHolder, _ := strconv.ParseFloat(getEnv("MAX_TOP_HOLDER_PERCENT", "10"), 64)
	maxDevWallet, _ := strconv.ParseFloat(getEnv("MAX_DEV_WALLET_PERCENT", "5"), 64)
	minHolders, _ := strconv.Atoi(getEnv("MIN_HOLDER_COUNT", "100"))

	stopLoss, _ := strconv.ParseFloat(getEnv("STOP_LOSS_PERCENT", "0.5"), 64)
	trailingStop, _ := strconv.ParseFloat(getEnv("TRAILING_STOP_PERCENT", "0.2"), 64)
	timeout, _ := strconv.Atoi(getEnv("TIMEOUT_MINUTES", "30"))

	maxTrades, _ := strconv.Atoi(getEnv("MAX_TRADES_PER_HOUR", "10"))
	cooldown, _ := strconv.Atoi(getEnv("COOLDOWN_SECONDS", "30"))
	signalMaxAge, _ := strconv.Atoi(getEnv("SIGNAL_MAX_AGE_MINUTES", "3"))

	distThreshold, _ := strconv.ParseFloat(getEnv("DISTRIBUTION_THRESHOLD", "0.7"), 64)

	return &Config{
		// Mode
		PaperTrading:       getEnv("PAPER_TRADING", "true") == "true",

		// Telegram
		TelegramAPIID:      apiID,
		TelegramAPIHash:    getEnv("TELEGRAM_API_HASH", ""),
		TelegramBotToken:   getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:     chatID,
		TelegramPhone:      getEnv("TELEGRAM_PHONE", ""),
		MonitoredChannels:  splitEnv("MONITORED_CHANNELS", ","),

		// Solana
		SolanaRPCURL:        getEnv("SOLANA_RPC_URL", "https://api.mainnet-beta.solana.com"),
		SolanaWSURL:         getEnv("SOLANA_WS_URL", "wss://api.mainnet-beta.solana.com"),
		AdditionalRPCURLs:   splitEnv("SOLANA_ADDITIONAL_RPC_URLS", ","),
		PrivateKey:          getEnv("SOLANA_PRIVATE_KEY", ""),

		// Trading
		PortfolioSize:      portfolioSize,
		RiskPerTrade:       riskPerTrade,
		MaxPositionSize:    maxPosition,
		MinLiquidity:       minLiquidity,
		SlippageBPS:        slippage,
		PriorityFeeLamports: priorityFee,

		MaxSlippagePercent:  maxSlipPct,
		MaxLpSharePercent:   maxLpSharePct,
		MaxLpDropPercent:    maxLpDropPct,
		LpDropWindowSeconds: lpDropWindowSecs,

		// Safety
		MinLiquidityLocked:   minLPLocked,
		MaxTopHolderPercent:  maxTopHolder,
		MaxDevWalletPercent:  maxDevWallet,
		MinHolderCount:       minHolders,
		MaxMintAuthority:     getEnv("REJECT_MINT_AUTHORITY", "true") == "true",
		MaxFreezeAuthority:   getEnv("REJECT_FREEZE_AUTHORITY", "true") == "true",

		// Exit Strategy
		TakeProfitLevels:    parseFloatSlice(getEnv("TAKE_PROFIT_LEVELS", "2.0,5.0,10.0")),
		TakeProfitPercents:  parseFloatSlice(getEnv("TAKE_PROFIT_PERCENTS", "0.3,0.3,0.4")),
		StopLossPercent:     stopLoss,
		TrailingStopPercent: trailingStop,
		TimeoutMinutes:      timeout,

		// Wallet Tracking
		TrackedWallets:        splitEnv("TRACKED_WALLETS", ","),
		WalletTrackingEnabled: getEnv("WALLET_TRACKING_ENABLED", "true") == "true",

		// Twitter
		TwitterBearerToken:  getEnv("TWITTER_BEARER_TOKEN", ""),
		TwitterEnabled:      getEnv("TWITTER_ENABLED", "false") == "true",
		TwitterKeywords:     splitEnv("TWITTER_KEYWORDS", ","),

		// API Keys
		BirdeyeAPIKey:       getEnv("BIRDEYE_API_KEY", ""),
		HeliusAPIKey:        getEnv("HELIUS_API_KEY", ""),

		// Rate Limiting
		MaxTradesPerHour:    maxTrades,
		CooldownSeconds:     cooldown,
		SignalMaxAgeMinutes: signalMaxAge,

		// Flow / distribution exits
		DistributionThreshold: distThreshold,
	}, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func splitEnv(key, sep string) []string {
	val := os.Getenv(key)
	if val == "" {
		return []string{}
	}
	parts := strings.Split(val, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseFloatSlice(s string) []float64 {
	parts := strings.Split(s, ",")
	result := make([]float64, 0, len(parts))
	for _, p := range parts {
		if v, err := strconv.ParseFloat(strings.TrimSpace(p), 64); err == nil {
			result = append(result, v)
		}
	}
	return result
}
