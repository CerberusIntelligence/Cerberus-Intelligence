package walletscanner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"solana-trading-bot/config"

	log "github.com/sirupsen/logrus"
)

type WalletScanner struct {
	cfg    *config.Config
	client *http.Client
	sem    chan struct{} // limits to 1 concurrent discovery
}

func New(cfg *config.Config) *WalletScanner {
	return &WalletScanner{
		cfg:    cfg,
		client: &http.Client{Timeout: 15 * time.Second},
		sem:    make(chan struct{}, 1),
	}
}

type heliusTx struct {
	Signature      string          `json:"signature"`
	Timestamp      int64           `json:"timestamp"`
	Type           string          `json:"type"`
	TokenTransfers []tokenTransfer `json:"tokenTransfers"`
}

type tokenTransfer struct {
	FromUserAccount string  `json:"fromUserAccount"`
	ToUserAccount   string  `json:"toUserAccount"`
	Mint            string  `json:"mint"`
	TokenAmount     float64 `json:"tokenAmount"`
}

// DiscoverFromToken finds wallets that sold this token during its pump.
// Those wallets bought early and took profit — they have real edge.
func (ws *WalletScanner) DiscoverFromToken(ctx context.Context, tokenMint string) []string {
	// Only one discovery at a time to avoid rate limiting
	select {
	case ws.sem <- struct{}{}:
		defer func() { <-ws.sem }()
	default:
		log.Debug("Wallet discovery: skipping — another discovery in progress")
		return nil
	}

	txs, err := ws.getTransactions(ctx, tokenMint, 100)
	if err != nil {
		log.WithError(err).Warn("Wallet discovery: failed to fetch transactions")
		return nil
	}

	// Find wallets that SOLD this token (fromUserAccount sent the token = seller)
	sellerSeen := make(map[string]bool)
	var sellers []string
	for _, tx := range txs {
		for _, tt := range tx.TokenTransfers {
			if tt.Mint != tokenMint || tt.FromUserAccount == "" {
				continue
			}
			if !sellerSeen[tt.FromUserAccount] {
				sellerSeen[tt.FromUserAccount] = true
				sellers = append(sellers, tt.FromUserAccount)
			}
		}
	}

	if len(sellers) == 0 {
		log.WithField("mint", tokenMint[:8]+"...").Debug("Wallet discovery: no sellers found")
		return nil
	}

	log.WithFields(log.Fields{
		"mint":       tokenMint[:8] + "...",
		"candidates": len(sellers),
	}).Info("Wallet discovery: scoring candidates")

	type scored struct {
		addr  string
		score float64
	}
	var results []scored

	// Score only top 5 candidates to stay within rate limits
	limit := len(sellers)
	if limit > 5 {
		limit = 5
	}
	for _, addr := range sellers[:limit] {
		time.Sleep(500 * time.Millisecond) // pace API calls
		score, err := ws.scoreWallet(ctx, addr)
		if err != nil || score < 0.4 {
			continue
		}
		results = append(results, scored{addr, score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	var top []string
	for i, r := range results {
		if i >= 5 {
			break
		}
		top = append(top, r.addr)
		log.WithFields(log.Fields{
			"wallet": r.addr[:8] + "...",
			"score":  fmt.Sprintf("%.2f", r.score),
		}).Info("Wallet discovery: found profitable wallet")
	}
	return top
}

// scoreWallet scores a wallet 0–1 based on trading patterns.
// Higher score = more likely to be an active memecoin trader with edge.
func (ws *WalletScanner) scoreWallet(ctx context.Context, addr string) (float64, error) {
	txs, err := ws.getTransactions(ctx, addr, 30)
	if err != nil {
		return 0, err
	}

	swapCount := 0
	pumpTokens := 0
	recentActivity := false
	cutoff24h := time.Now().Add(-24 * time.Hour).Unix()

	for _, tx := range txs {
		if tx.Type != "SWAP" {
			continue
		}
		swapCount++
		if tx.Timestamp > cutoff24h {
			recentActivity = true
		}
		for _, tt := range tx.TokenTransfers {
			if strings.HasSuffix(tt.Mint, "pump") {
				pumpTokens++
				break
			}
		}
	}

	if swapCount == 0 {
		return 0, nil
	}

	// Score:
	// Active trader (10+ swaps)   = up to 0.3
	// Recent activity (last 24h)  = 0.3
	// Trades pump.fun tokens      = up to 0.4
	score := 0.0
	if swapCount >= 10 {
		score += 0.3
	} else if swapCount >= 5 {
		score += 0.15
	}
	if recentActivity {
		score += 0.3
	}
	pumpRatio := float64(pumpTokens) / float64(swapCount)
	score += pumpRatio * 0.4

	return score, nil
}

func (ws *WalletScanner) getTransactions(ctx context.Context, address string, limit int) ([]heliusTx, error) {
	url := fmt.Sprintf(
		"https://api.helius.xyz/v0/addresses/%s/transactions?api-key=%s&limit=%d",
		address, ws.cfg.HeliusAPIKey, limit,
	)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := ws.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	// Helius returns an error object {} on failure, array [] on success
	if len(raw) == 0 || raw[0] != '[' {
		return nil, fmt.Errorf("helius error: %s", string(raw))
	}
	var txs []heliusTx
	if err := json.Unmarshal(raw, &txs); err != nil {
		return nil, err
	}
	return txs, nil
}
