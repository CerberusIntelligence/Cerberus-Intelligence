package models

// MonthlyStats tracks win/loss for one calendar month window.
type MonthlyStats struct {
	YearMonth string  `json:"year_month"` // e.g. "2024-11"
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
	WinRate   float64 `json:"win_rate"`
}

// WalletAnalysis is the full enriched analysis result for one wallet.
type WalletAnalysis struct {
	Address string

	// From Birdeye (aggregate)
	BirdeyePnL    float64
	BirdeyeTrades int
	PeriodCount   int // how many leaderboard periods this wallet appeared in

	// Calculated from Helius transaction history
	BirdeyeWinRate  float64 // computed from Helius position matching
	TotalPnLSOL     float64 // realised PnL in SOL from matched positions
	WinCount        int     // number of profitable closed positions
	LossCount       int     // number of losing closed positions
	WinDays         int     // distinct calendar days that had at least one win
	WinWeeks        int     // distinct calendar weeks that had at least one win
	AvgWinSOL       float64 // average SOL profit per winning trade
	AvgLossSOL      float64 // average SOL loss per losing trade (absolute value)
	TopWinPct       float64 // % of total PnL from the single biggest win (scraper signal)

	// From Helius (transaction history)
	HistoryDays      int     // days between first and last observed trade
	DaysSinceActive  int     // days since the last trade
	SwapCount        int     // total SWAP txs fetched from Helius
	AvgHoldSeconds   float64 // average hold time of winning positions in seconds
	MonthlyStats     []MonthlyStats

	// Derived scoring components (each 0–1)
	ConsistencyScore float64 // stability of monthly win rates
	HistoryScore     float64 // depth of history (capped at 180d = 1.0)
	RecencyScore     float64 // decays linearly over 30 idle days

	// Final composite score (0–100)
	Score float64
}

// RankedWallet is the output record for a top wallet.
type RankedWallet struct {
	Rank             int            `json:"rank"`
	Address          string         `json:"address"`
	Score            float64        `json:"score"`
	WinRate          float64        `json:"win_rate_pct"`
	TotalPnLUSD      float64        `json:"total_pnl_usd"`
	TotalTrades      int            `json:"total_trades"`
	WinCount         int            `json:"win_count"`
	WinDays          int            `json:"win_days"`
	WinWeeks         int            `json:"win_weeks"`
	AvgWinSOL        float64        `json:"avg_win_sol"`
	AvgLossSOL       float64        `json:"avg_loss_sol"`
	TopWinPct        float64        `json:"top_win_pct"`
	AvgHoldSeconds   float64        `json:"avg_hold_seconds"`
	PeriodCount      int            `json:"period_count"`
	ConsistencyScore float64        `json:"consistency_score"`
	HistoryDays      int            `json:"history_days"`
	DaysSinceActive  int            `json:"days_since_active"`
	MonthlyWinRates  []MonthlyStats `json:"monthly_win_rates,omitempty"`
}
