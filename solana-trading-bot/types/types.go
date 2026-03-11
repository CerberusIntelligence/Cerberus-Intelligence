package types

import "time"

type Signal struct {
	Address   string
	Source    string
	Message   string
	Timestamp time.Time
	IsSell    bool    // true = wallet sold this token, close our position
	Price     float64 // exact execution price derived from tx SOL/token balance delta
}

type TokenInfo struct {
	Address       string
	Symbol        string
	Name          string
	PriceSOL      float64
	PriceUSD      float64
	LiquiditySOL  float64
	LiquidityUSD  float64
	Volume24h     float64
	PriceChange5m float64
	MintRevoked   bool
	FreezeRevoked bool
	CreatedAt     time.Time
}

type Position struct {
	Address       string
	Symbol        string
	EntryPrice    float64
	CurrentPrice  float64
	HighestPrice  float64
	Quantity      float64
	EntryValueSOL float64
	OpenedAt      time.Time
	Source        string
}

type Trade struct {
	Address       string
	Symbol        string
	Side          string
	EntryPrice    float64
	ExitPrice     float64
	Quantity      float64
	ValueSOL      float64
	PnLSOL        float64
	PnLPct        float64
	Reason        string
	Source        string
	OpenedAt      time.Time
	ClosedAt      time.Time
}
