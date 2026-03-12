package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HeliusClient queries the Helius enhanced transaction API.
type HeliusClient struct {
	apiKey string
	http   *http.Client
}

func NewHelius(apiKey string) *HeliusClient {
	return &HeliusClient{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

// HeliusTx is the minimal enhanced transaction shape we care about.
type HeliusTx struct {
	Signature      string           `json:"signature"`
	Timestamp      int64            `json:"timestamp"`
	Type           string           `json:"type"`
	TokenTransfers []TokenTransfer  `json:"tokenTransfers"`
	AccountData    []AccountData    `json:"accountData"`
}

// TokenTransfer represents a token movement in a transaction.
type TokenTransfer struct {
	FromUserAccount string  `json:"fromUserAccount"`
	ToUserAccount   string  `json:"toUserAccount"`
	Mint            string  `json:"mint"`
	TokenAmount     float64 `json:"tokenAmount"`
}

// AccountData holds SOL balance change for an account.
type AccountData struct {
	Account         string `json:"account"`
	NativeBalChange int64  `json:"nativeBalanceChange"` // lamports
}

// GetSwapTransactions fetches up to `limit` SWAP transactions for an address.
// Pages through Helius using the `before` cursor to get oldest history possible.
func (h *HeliusClient) GetSwapTransactions(ctx context.Context, address string, limit int) ([]HeliusTx, error) {
	if h.apiKey == "" {
		return nil, fmt.Errorf("HELIUS_API_KEY not set")
	}

	var all []HeliusTx
	before := ""
	batchSize := 100
	if batchSize > limit {
		batchSize = limit
	}

	for len(all) < limit {
		batch, err := h.fetchBatch(ctx, address, batchSize, before)
		if err != nil {
			return all, err
		}
		if len(batch) == 0 {
			break // no more history
		}

		// Filter for SWAP transactions only
		for _, tx := range batch {
			if tx.Type == "SWAP" {
				all = append(all, tx)
			}
		}

		// Cursor for next page = last signature of this batch
		before = batch[len(batch)-1].Signature

		// Helius returns fewer than batchSize when we've hit the end
		if len(batch) < batchSize {
			break
		}

		// Respect rate limits (free tier: ~10 req/s)
		select {
		case <-ctx.Done():
			return all, ctx.Err()
		case <-time.After(120 * time.Millisecond):
		}
	}

	return all, nil
}

func (h *HeliusClient) fetchBatch(ctx context.Context, address string, limit int, before string) ([]HeliusTx, error) {
	url := fmt.Sprintf(
		"https://api.helius.xyz/v0/addresses/%s/transactions?api-key=%s&limit=%d&type=SWAP",
		address, h.apiKey, limit,
	)
	if before != "" {
		url += "&before=" + before
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("helius request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		// Rate limited — back off and retry once
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
		resp, err = h.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("helius HTTP %d for %s", resp.StatusCode, address[:8]+"...")
	}

	// Helius returns [] on success, {} on error
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("helius decode error: %w", err)
	}
	if len(raw) == 0 || raw[0] != '[' {
		return nil, fmt.Errorf("helius error response: %s", string(raw))
	}

	var txs []HeliusTx
	if err := json.Unmarshal(raw, &txs); err != nil {
		return nil, fmt.Errorf("helius unmarshal error: %w", err)
	}
	return txs, nil
}
