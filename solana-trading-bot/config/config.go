package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// Mode
	PaperTrading bool

	// Telegram
	TelegramBotToken string
	TelegramChatID   int64

	// Solana
	SolanaRPCURL      string
	SolanaWSURL       string
	HeliusAPIKey      string
	PrivateKey        string
	WalletPrivKey     string   // alias used by standalone jupiter package
	AdditionalRPCURLs []string // extra RPC endpoints for tx blasting

	// Trading
	PaperBalance        float64 // Starting paper balance in SOL
	TradeAmountSOL      float64 // SOL per trade
	MaxPositions        int     // Max concurrent positions
	MinLiquidityUSD     float64 // Minimum pool liquidity to enter
	SlippageBPS         int
	PriorityFeeLamports uint64
	PriorityFee         uint64  // alias used by standalone jupiter package
	MaxSlippagePercent  float64 // abort buy if projected price impact exceeds this %

	// Wallet tracking / copy trading
	WalletTrackingEnabled bool
	TrackedWallets        []string

	// Safety
	RequireMintRevoked   bool
	RequireFreezeRevoked bool

	// Exit strategy
	StopLossPct     float64
	TrailingStopPct float64
	TimeoutMinutes  int
	TakeProfit1x    float64 // multiplier, e.g. 1.5 = 1.5x
	TakeProfit2x    float64
	TakeProfit3x    float64
	TP1Pct          float64 // fraction to sell at TP1, e.g. 0.33
	TP2Pct          float64
	TP3Pct          float64

	// Rate limiting
	MaxTradesPerHour int
	CooldownSeconds  int
}

func Load() *Config {
	godotenv.Load()

	chatID, _ := strconv.ParseInt(getEnv("TELEGRAM_CHAT_ID", "0"), 10, 64)
	paperBalance, _ := strconv.ParseFloat(getEnv("PORTFOLIO_SIZE_SOL", "3.5"), 64)
	tradeAmount, _ := strconv.ParseFloat(getEnv("MAX_POSITION_SOL", "0.07"), 64)
	maxPositions, _ := strconv.Atoi(getEnv("MAX_POSITIONS", "10"))
	minLiquidity, _ := strconv.ParseFloat(getEnv("MIN_LIQUIDITY_USD", "500"), 64)
	slippage, _ := strconv.Atoi(getEnv("SLIPPAGE_BPS", "500"))
	priorityFee, _ := strconv.ParseUint(getEnv("PRIORITY_FEE_LAMPORTS", "1000000"), 10, 64)
	maxSlippage, _ := strconv.ParseFloat(getEnv("MAX_SLIPPAGE_PERCENT", "0"), 64)
	walletTracking := getEnv("WALLET_TRACKING_ENABLED", "false") == "true"
	trackedWallets := splitCSV(getEnv("TRACKED_WALLETS", ""))
	additionalRPCs := splitCSV(getEnv("ADDITIONAL_RPC_URLS", ""))
	stopLoss, _ := strconv.ParseFloat(getEnv("STOP_LOSS_PERCENT", "0.30"), 64)
	trailingStop, _ := strconv.ParseFloat(getEnv("TRAILING_STOP_PERCENT", "0.25"), 64)
	timeout, _ := strconv.Atoi(getEnv("TIMEOUT_MINUTES", "30"))
	maxTrades, _ := strconv.Atoi(getEnv("MAX_TRADES_PER_HOUR", "10"))
	cooldown, _ := strconv.Atoi(getEnv("COOLDOWN_SECONDS", "30"))

	tpLevels := parseFloatSlice(getEnv("TAKE_PROFIT_LEVELS", "1.5,3.0,7.0"))
	tpPcts := parseFloatSlice(getEnv("TAKE_PROFIT_PERCENTS", "0.33,0.33,0.34"))

	tp1x, tp2x, tp3x := 1.5, 3.0, 7.0
	if len(tpLevels) >= 3 {
		tp1x, tp2x, tp3x = tpLevels[0], tpLevels[1], tpLevels[2]
	}
	tp1pct, tp2pct, tp3pct := 0.33, 0.33, 0.34
	if len(tpPcts) >= 3 {
		tp1pct, tp2pct, tp3pct = tpPcts[0], tpPcts[1], tpPcts[2]
	}

	return &Config{
		PaperTrading: getEnv("PAPER_TRADING", "true") == "true",

		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:   chatID,

		SolanaRPCURL:      getEnv("SOLANA_RPC_URL", "https://api.mainnet-beta.solana.com"),
		SolanaWSURL:       getEnv("SOLANA_WS_URL", "wss://api.mainnet-beta.solana.com"),
		HeliusAPIKey:      getEnv("HELIUS_API_KEY", ""),
		PrivateKey:        getEnv("SOLANA_PRIVATE_KEY", ""),
		WalletPrivKey:     getEnv("SOLANA_PRIVATE_KEY", ""),
		AdditionalRPCURLs: additionalRPCs,

		PaperBalance:        paperBalance,
		TradeAmountSOL:      tradeAmount,
		MaxPositions:        maxPositions,
		MinLiquidityUSD:     minLiquidity,
		SlippageBPS:         slippage,
		PriorityFeeLamports: priorityFee,
		PriorityFee:         priorityFee,
		MaxSlippagePercent:  maxSlippage,

		WalletTrackingEnabled: walletTracking,
		TrackedWallets:        trackedWallets,

		RequireMintRevoked:   getEnv("REJECT_MINT_AUTHORITY", "false") == "true",
		RequireFreezeRevoked: getEnv("REJECT_FREEZE_AUTHORITY", "false") == "true",

		StopLossPct:     stopLoss,
		TrailingStopPct: trailingStop,
		TimeoutMinutes:  timeout,
		TakeProfit1x:    tp1x,
		TakeProfit2x:    tp2x,
		TakeProfit3x:    tp3x,
		TP1Pct:          tp1pct,
		TP2Pct:          tp2pct,
		TP3Pct:          tp3pct,

		MaxTradesPerHour: maxTrades,
		CooldownSeconds:  cooldown,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
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
