package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// API keys
	BirdeyeAPIKey string
	HeliusAPIKey  string

	// Telegram
	TelegramToken  string
	TelegramChatID string

	// Filter thresholds
	MinTrades        int     // minimum completed trades in history (Birdeye)
	MinWinCount      int     // minimum number of actual winning positions (Helius)
	MinWinDays       int     // wins must be spread across at least this many days
	MaxTopWinPct     float64 // reject if single biggest win > this % of total PnL (scraper filter)
	MinWinRate       float64 // minimum win rate 0.0–1.0 (e.g. 0.55 = 55%)
	MinHistoryDays   int     // wallet must have been trading at least this long
	MaxActiveAgoDays int     // must have traded within this many days (recency gate)
	MinProfitFactor  float64 // total_wins / abs(total_losses) — must exceed 1.0
	MinPeriodPnLUSD  float64 // minimum Birdeye period PnL in USD (filters low/negative all-time wallets)
	MinHeliusPnLSOL  float64 // minimum SOL profit from Helius-analyzed trades (must be positive)
	MinHoldSeconds   int     // minimum hold time in seconds for a win to count (filters bots/flippers)

	// Discovery
	DiscoveryBatchSize int // how many candidates to pull from each leaderboard period

	// Analysis depth
	HeliusTxLimit int // max transactions to fetch per wallet from Helius

	// Output
	TopN           int    // number of top wallets to display/save
	OutputFile     string // JSON results file
	ExportForBot   bool   // also write bot-compatible tracked_wallets.json
	BotWalletsFile string // path to bot's tracked_wallets.json
}

func Load() *Config {
	godotenv.Load()

	minTrades, _ := strconv.Atoi(getEnv("MIN_TRADES", "30"))
	minWinCount, _ := strconv.Atoi(getEnv("MIN_WIN_COUNT", "5"))
	minWinDays, _ := strconv.Atoi(getEnv("MIN_WIN_DAYS", "2"))
	maxTopWinPct, _ := strconv.ParseFloat(getEnv("MAX_TOP_WIN_PCT", "0.80"), 64)
	minWinRate, _ := strconv.ParseFloat(getEnv("MIN_WIN_RATE", "0.55"), 64)
	minHistoryDays, _ := strconv.Atoi(getEnv("MIN_HISTORY_DAYS", "0"))
	maxActiveAgoDays, _ := strconv.Atoi(getEnv("MAX_ACTIVE_AGO_DAYS", "3"))
	minProfitFactor, _ := strconv.ParseFloat(getEnv("MIN_PROFIT_FACTOR", "1.2"), 64)
	minPeriodPnLUSD, _ := strconv.ParseFloat(getEnv("MIN_PERIOD_PNL_USD", "50000"), 64)
	minHeliusPnLSOL, _ := strconv.ParseFloat(getEnv("MIN_HELIUS_PNL_SOL", "5"), 64)
	minHoldSeconds, _ := strconv.Atoi(getEnv("MIN_HOLD_SECONDS", "60"))
	discoveryBatch, _ := strconv.Atoi(getEnv("DISCOVERY_BATCH", "200"))
	heliusTxLimit, _ := strconv.Atoi(getEnv("HELIUS_TX_LIMIT", "500"))
	topN, _ := strconv.Atoi(getEnv("TOP_N", "20"))

	return &Config{
		BirdeyeAPIKey:      getEnv("BIRDEYE_API_KEY", ""),
		HeliusAPIKey:       getEnv("HELIUS_API_KEY", ""),
		TelegramToken:      getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:     getEnv("TELEGRAM_CHAT_ID", ""),
		MinTrades:          minTrades,
		MinWinCount:        minWinCount,
		MinWinDays:         minWinDays,
		MaxTopWinPct:       maxTopWinPct,
		MinWinRate:         minWinRate,
		MinHistoryDays:     minHistoryDays,
		MaxActiveAgoDays:   maxActiveAgoDays,
		MinProfitFactor:    minProfitFactor,
		MinPeriodPnLUSD:    minPeriodPnLUSD,
		MinHeliusPnLSOL:    minHeliusPnLSOL,
		MinHoldSeconds:     minHoldSeconds,
		DiscoveryBatchSize: discoveryBatch,
		HeliusTxLimit:      heliusTxLimit,
		TopN:               topN,
		OutputFile:         getEnv("OUTPUT_FILE", "top_wallets.json"),
		ExportForBot:       getEnv("EXPORT_FOR_BOT", "false") == "true",
		BotWalletsFile:     getEnv("BOT_WALLETS_FILE", "../solana-trading-bot/tracked_wallets.json"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
