package types

import "time"

// Trade type / side / status string constants (used by solana/jupiter.go)
const (
	TradeMarket   = "market"
	TradeBuy      = "buy"
	TradeSell     = "sell"
	TradeExecuted = "executed"
	TradeFailed   = "failed"
)

// Token represents an SPL token's on-chain metadata.
type Token struct {
	Address      string
	Symbol       string
	Name         string
	Decimals     int
	Supply       uint64
	DiscoveredAt time.Time
}

// WalletActivity is a swap event detected on a copy-traded wallet.
type WalletActivity struct {
	Wallet       string
	TokenAddress string
	Action       string // "buy" or "sell"
	AmountSOL    float64
	TokenAmount  float64
	TxSignature  string
	Timestamp    time.Time
}

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
	// Engine-level fields (closed trade record)
	Address    string
	Symbol     string
	Side       string
	EntryPrice float64
	ExitPrice  float64
	Quantity   float64
	ValueSOL   float64
	PnLSOL     float64
	PnLPct     float64
	Reason     string
	Source     string
	OpenedAt   time.Time
	ClosedAt   time.Time

	// Execution-level fields (set by solana/jupiter.go)
	Token       *Token
	Type        string
	Price       float64
	Status      string
	TxSignature string
	Error       string
	ExecutedAt  time.Time
}
