package solana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"solana-trading-bot/config"
	"solana-trading-bot/types"

	"github.com/sirupsen/logrus"
)

const (
	JupiterQuoteAPI = "https://quote-api.jup.ag/v6/quote"
	JupiterSwapAPI  = "https://quote-api.jup.ag/v6/swap"
)

// Jupiter handles swap operations via Jupiter aggregator
type Jupiter struct {
	cfg        *config.Config
	log        *logrus.Logger
	client     *Client
	httpClient *http.Client
	tokenCache sync.Map // address -> *types.Token; avoids redundant GetTokenInfo calls
}

// NewJupiter creates a new Jupiter client
func NewJupiter(cfg *config.Config, log *logrus.Logger, client *Client) *Jupiter {
	return &Jupiter{
		cfg:    cfg,
		log:    log,
		client: client,
		httpClient: &http.Client{
			Timeout:   8 * time.Second,
			Transport: sharedTransport, // reuse shared transport from client.go
		},
	}
}

// CacheToken pre-populates token metadata so Buy/Sell don't need to re-fetch it.
// The engine calls this after fetching token info, before executing a trade.
func (j *Jupiter) CacheToken(token *types.Token) {
	if token != nil && token.Address != "" {
		j.tokenCache.Store(token.Address, token)
	}
}

// getCachedToken retrieves a previously cached token, or nil if not found.
func (j *Jupiter) getCachedToken(address string) *types.Token {
	if v, ok := j.tokenCache.Load(address); ok {
		return v.(*types.Token)
	}
	return nil
}

// QuoteRequest for Jupiter API
type QuoteRequest struct {
	InputMint           string `json:"inputMint"`
	OutputMint          string `json:"outputMint"`
	Amount              uint64 `json:"amount"`
	SlippageBps         int    `json:"slippageBps"`
	OnlyDirectRoutes    bool   `json:"onlyDirectRoutes,omitempty"`
	AsLegacyTransaction bool   `json:"asLegacyTransaction,omitempty"`
}

// QuoteResponse from Jupiter API
type QuoteResponse struct {
	InputMint            string          `json:"inputMint"`
	InAmount             string          `json:"inAmount"`
	OutputMint           string          `json:"outputMint"`
	OutAmount            string          `json:"outAmount"`
	OtherAmountThreshold string          `json:"otherAmountThreshold"`
	SwapMode             string          `json:"swapMode"`
	SlippageBps          int             `json:"slippageBps"`
	PriceImpactPct       string          `json:"priceImpactPct"`
	RoutePlan            json.RawMessage `json:"routePlan"`
}

// SwapRequest for Jupiter swap
type SwapRequest struct {
	QuoteResponse             json.RawMessage `json:"quoteResponse"`
	UserPublicKey             string          `json:"userPublicKey"`
	WrapAndUnwrapSol          bool            `json:"wrapAndUnwrapSol"`
	UseSharedAccounts         bool            `json:"useSharedAccounts"`
	PrioritizationFeeLamports interface{}     `json:"prioritizationFeeLamports,omitempty"`
	AsLegacyTransaction       bool            `json:"asLegacyTransaction"`
	DynamicComputeUnitLimit   bool            `json:"dynamicComputeUnitLimit"`
}

// SwapResponse from Jupiter API
type SwapResponse struct {
	SwapTransaction      string `json:"swapTransaction"`
	LastValidBlockHeight uint64 `json:"lastValidBlockHeight"`
}

// GetQuote fetches a quote for a swap
func (j *Jupiter) GetQuote(ctx context.Context, inputMint, outputMint string, amount uint64) (*QuoteResponse, error) {
	url := fmt.Sprintf("%s?inputMint=%s&outputMint=%s&amount=%d&slippageBps=%d&onlyDirectRoutes=false",
		JupiterQuoteAPI, inputMint, outputMint, amount, j.cfg.SlippageBPS)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("quote request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("quote failed: %s - %s", resp.Status, string(body))
	}

	var quote QuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return nil, fmt.Errorf("failed to decode quote: %w", err)
	}

	return &quote, nil
}

// GetSwapTransaction builds the swap transaction
func (j *Jupiter) GetSwapTransaction(ctx context.Context, quote *QuoteResponse) (*SwapResponse, error) {
	quoteJSON, err := json.Marshal(quote)
	if err != nil {
		return nil, err
	}

	priorityFee := map[string]interface{}{
		"priorityLevelWithMaxLamports": map[string]interface{}{
			"maxLamports":   j.cfg.PriorityFeeLamports,
			"priorityLevel": "veryHigh",
		},
	}

	swapReq := SwapRequest{
		QuoteResponse:             quoteJSON,
		UserPublicKey:             j.client.GetWalletAddress(),
		WrapAndUnwrapSol:          true,
		UseSharedAccounts:         true,
		PrioritizationFeeLamports: priorityFee,
		AsLegacyTransaction:       false,
		DynamicComputeUnitLimit:   true,
	}

	body, err := json.Marshal(swapReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", JupiterSwapAPI, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("swap request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("swap failed: %s - %s", resp.Status, string(respBody))
	}

	var swapResp SwapResponse
	if err := json.NewDecoder(resp.Body).Decode(&swapResp); err != nil {
		return nil, fmt.Errorf("failed to decode swap: %w", err)
	}

	return &swapResp, nil
}

// Buy executes a buy order (SOL -> Token)
func (j *Jupiter) Buy(ctx context.Context, tokenMint string, amountSOL float64) (*types.Trade, error) {
	start := time.Now()

	j.log.WithFields(logrus.Fields{
		"token":  tokenMint,
		"amount": amountSOL,
	}).Info("Executing buy order")

	amountLamports := uint64(amountSOL * 1e9)

	quote, err := j.GetQuote(ctx, WSOL, tokenMint, amountLamports)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}

	// Enforce projected slippage guard
	if j.cfg.MaxSlippagePercent > 0 {
		var rawImpact float64
		fmt.Sscanf(quote.PriceImpactPct, "%f", &rawImpact)
		impactPct := rawImpact
		if impactPct < 1 {
			impactPct *= 100
		}
		if impactPct > j.cfg.MaxSlippagePercent {
			j.log.WithFields(logrus.Fields{
				"token":            tokenMint,
				"impact_pct":       impactPct,
				"max_slippage_pct": j.cfg.MaxSlippagePercent,
			}).Warn("Aborting buy: projected slippage above threshold")
			return nil, fmt.Errorf("projected slippage %.2f%% exceeds max %.2f%%", impactPct, j.cfg.MaxSlippagePercent)
		}
	}

	swap, err := j.GetSwapTransaction(ctx, quote)
	if err != nil {
		return nil, fmt.Errorf("failed to get swap tx: %w", err)
	}

	buildDone := time.Now()

	sig, err := j.client.SendTransaction(ctx, swap.SwapTransaction)
	if err != nil {
		return nil, fmt.Errorf("failed to send tx: %w", err)
	}

	sentAt := time.Now()

	j.log.WithFields(logrus.Fields{
		"signature":         sig,
		"detect_to_sent_ms": sentAt.Sub(start).Milliseconds(),
		"build_ms":          buildDone.Sub(start).Milliseconds(),
	}).Info("Buy transaction sent")

	if err := j.client.WaitForConfirmation(ctx, sig, 60*time.Second); err != nil {
		return &types.Trade{
			Token:       &types.Token{Address: tokenMint},
			Type:        types.TradeMarket,
			Side:        types.TradeBuy,
			ValueSOL:    amountSOL,
			TxSignature: sig,
			Status:      types.TradeFailed,
			Error:       err.Error(),
			ExecutedAt:  time.Now(),
		}, err
	}

	confirmedAt := time.Now()

	var outAmount uint64
	fmt.Sscanf(quote.OutAmount, "%d", &outAmount)

	// Use cached token info (pre-populated by engine before trade) — avoids a redundant RPC call.
	// The engine overwrites Trade.Token anyway, so this is only used for decimals + quantity calc.
	tokenInfo := j.getCachedToken(tokenMint)
	if tokenInfo == nil {
		tokenInfo = &types.Token{Address: tokenMint, Symbol: "UNKNOWN", Decimals: 9}
	}

	quantity := float64(outAmount) / float64(pow10(uint8(tokenInfo.Decimals)))
	price := amountSOL / quantity

	var rawImpact float64
	fmt.Sscanf(quote.PriceImpactPct, "%f", &rawImpact)
	expectedSlipPct := rawImpact
	if expectedSlipPct < 1 {
		expectedSlipPct *= 100
	}

	j.log.WithFields(logrus.Fields{
		"signature":             sig,
		"detect_to_sent_ms":     sentAt.Sub(start).Milliseconds(),
		"sent_to_confirm_ms":    confirmedAt.Sub(sentAt).Milliseconds(),
		"expected_slippage_pct": expectedSlipPct,
	}).Info("Buy confirmed")

	return &types.Trade{
		Token:       tokenInfo,
		Type:        types.TradeMarket,
		Side:        types.TradeBuy,
		Price:       price,
		Quantity:    quantity,
		ValueSOL:    amountSOL,
		TxSignature: sig,
		Status:      types.TradeExecuted,
		ExecutedAt:  time.Now(),
	}, nil
}

// Sell executes a sell order (Token -> SOL)
func (j *Jupiter) Sell(ctx context.Context, tokenMint string, amount float64, decimals int) (*types.Trade, error) {
	start := time.Now()

	j.log.WithFields(logrus.Fields{
		"token":  tokenMint,
		"amount": amount,
	}).Info("Executing sell order")

	amountSmallest := uint64(amount * float64(pow10(uint8(decimals))))

	quote, err := j.GetQuote(ctx, tokenMint, WSOL, amountSmallest)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}

	swap, err := j.GetSwapTransaction(ctx, quote)
	if err != nil {
		return nil, fmt.Errorf("failed to get swap tx: %w", err)
	}

	buildDone := time.Now()

	sig, err := j.client.SendTransaction(ctx, swap.SwapTransaction)
	if err != nil {
		return nil, fmt.Errorf("failed to send tx: %w", err)
	}

	sentAt := time.Now()

	j.log.WithFields(logrus.Fields{
		"signature":         sig,
		"detect_to_sent_ms": sentAt.Sub(start).Milliseconds(),
		"build_ms":          buildDone.Sub(start).Milliseconds(),
	}).Info("Sell transaction sent")

	if err := j.client.WaitForConfirmation(ctx, sig, 60*time.Second); err != nil {
		return &types.Trade{
			Token:       &types.Token{Address: tokenMint},
			Type:        types.TradeMarket,
			Side:        types.TradeSell,
			Quantity:    amount,
			TxSignature: sig,
			Status:      types.TradeFailed,
			Error:       err.Error(),
			ExecutedAt:  time.Now(),
		}, err
	}

	confirmedAt := time.Now()

	var outAmount uint64
	fmt.Sscanf(quote.OutAmount, "%d", &outAmount)
	valueSOL := float64(outAmount) / 1e9
	price := valueSOL / amount

	var rawImpact float64
	fmt.Sscanf(quote.PriceImpactPct, "%f", &rawImpact)
	expectedSlipPct := rawImpact
	if expectedSlipPct < 1 {
		expectedSlipPct *= 100
	}

	j.log.WithFields(logrus.Fields{
		"signature":             sig,
		"detect_to_sent_ms":     sentAt.Sub(start).Milliseconds(),
		"sent_to_confirm_ms":    confirmedAt.Sub(sentAt).Milliseconds(),
		"expected_slippage_pct": expectedSlipPct,
	}).Info("Sell confirmed")

	// Use cached token info; the engine overwrites Trade.Token but decimals are already known
	tokenInfo := j.getCachedToken(tokenMint)
	if tokenInfo == nil {
		tokenInfo = &types.Token{Address: tokenMint, Symbol: "UNKNOWN", Decimals: decimals}
	}

	return &types.Trade{
		Token:       tokenInfo,
		Type:        types.TradeMarket,
		Side:        types.TradeSell,
		Price:       price,
		Quantity:    amount,
		ValueSOL:    valueSOL,
		TxSignature: sig,
		Status:      types.TradeExecuted,
		ExecutedAt:  time.Now(),
	}, nil
}

// SellAll sells entire token balance
func (j *Jupiter) SellAll(ctx context.Context, tokenMint string) (*types.Trade, error) {
	balance, decimals, err := j.client.GetTokenBalance(ctx, tokenMint)
	if err != nil {
		return nil, err
	}

	if balance == 0 {
		return nil, fmt.Errorf("no balance to sell")
	}

	return j.Sell(ctx, tokenMint, balance, decimals)
}

// SellPercent sells a percentage of token balance
func (j *Jupiter) SellPercent(ctx context.Context, tokenMint string, percent float64) (*types.Trade, error) {
	balance, decimals, err := j.client.GetTokenBalance(ctx, tokenMint)
	if err != nil {
		return nil, err
	}

	if balance == 0 {
		return nil, fmt.Errorf("no balance to sell")
	}

	amount := balance * percent
	return j.Sell(ctx, tokenMint, amount, decimals)
}
