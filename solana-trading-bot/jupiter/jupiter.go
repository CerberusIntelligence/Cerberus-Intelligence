// Package jupiter handles live swap execution via Jupiter v6 API.
// This is only used when PAPER_TRADING=false.
package jupiter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	bin "github.com/gagliardetto/binary"
	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"solana-trading-bot/config"
)

const (
	SOLMint  = "So11111111111111111111111111111111111111112"
	quoteURL = "https://quote-api.jup.ag/v6/quote"
	swapURL  = "https://quote-api.jup.ag/v6/swap"
)

type Client struct {
	cfg    *config.Config
	http   *http.Client
	rpc    *rpc.Client
	wallet solana.PrivateKey
}

func New(cfg *config.Config) (*Client, error) {
	c := &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 30 * time.Second},
		rpc:  rpc.New(cfg.SolanaRPCURL),
	}

	if cfg.WalletPrivKey != "" {
		pk, err := solana.PrivateKeyFromBase58(cfg.WalletPrivKey)
		if err != nil {
			return nil, fmt.Errorf("invalid wallet private key: %w", err)
		}
		c.wallet = pk
	}

	return c, nil
}

type QuoteResponse struct {
	InputMint            string        `json:"inputMint"`
	InAmount             string        `json:"inAmount"`
	OutputMint           string        `json:"outputMint"`
	OutAmount            string        `json:"outAmount"`
	OtherAmountThreshold string        `json:"otherAmountThreshold"`
	SwapMode             string        `json:"swapMode"`
	SlippageBps          int           `json:"slippageBps"`
	PriceImpactPct       string        `json:"priceImpactPct"`
	RoutePlan            []interface{} `json:"routePlan"`
}

// GetQuote fetches a swap quote from Jupiter.
// buy=true  → SOL → token
// buy=false → token → SOL
func (c *Client) GetQuote(ctx context.Context, tokenMint string, amountLamports uint64, buy bool) (*QuoteResponse, error) {
	inputMint := SOLMint
	outputMint := tokenMint
	if !buy {
		inputMint = tokenMint
		outputMint = SOLMint
	}

	url := fmt.Sprintf("%s?inputMint=%s&outputMint=%s&amount=%d&slippageBps=%d",
		quoteURL, inputMint, outputMint, amountLamports, c.cfg.SlippageBPS)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var quote QuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return nil, err
	}
	return &quote, nil
}

// ExecuteSwap builds, signs, and submits a Jupiter swap transaction.
func (c *Client) ExecuteSwap(ctx context.Context, quote *QuoteResponse) (string, error) {
	if c.wallet == nil {
		return "", fmt.Errorf("wallet not configured (set SOLANA_PRIVATE_KEY)")
	}

	swapReq := map[string]interface{}{
		"quoteResponse":             quote,
		"userPublicKey":             c.wallet.PublicKey().String(),
		"wrapAndUnwrapSol":          true,
		"prioritizationFeeLamports": c.cfg.PriorityFee,
	}

	body, err := json.Marshal(swapReq)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", swapURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var swapResp struct {
		SwapTransaction      string `json:"swapTransaction"`
		LastValidBlockHeight uint64 `json:"lastValidBlockHeight"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&swapResp); err != nil {
		return "", err
	}

	// Decode base64 transaction
	txBytes, err := base64.StdEncoding.DecodeString(swapResp.SwapTransaction)
	if err != nil {
		return "", fmt.Errorf("decode tx: %w", err)
	}

	// Deserialize
	tx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(txBytes))
	if err != nil {
		return "", fmt.Errorf("deserialize tx: %w", err)
	}

	// Sign
	wallet := c.wallet
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(wallet.PublicKey()) {
			return &wallet
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("sign tx: %w", err)
	}

	// Submit
	sig, err := c.rpc.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentProcessed,
	})
	if err != nil {
		return "", fmt.Errorf("submit tx: %w", err)
	}

	return sig.String(), nil
}
