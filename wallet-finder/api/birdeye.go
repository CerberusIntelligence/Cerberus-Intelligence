package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BirdeyeClient queries the Birdeye public API.
type BirdeyeClient struct {
	apiKey string
	http   *http.Client
}

func NewBirdeye(apiKey string) *BirdeyeClient {
	return &BirdeyeClient{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 20 * time.Second},
	}
}

// birdeyeTraderItem matches the actual Birdeye free-tier response shape.
type birdeyeTraderItem struct {
	Address    string  `json:"address"`
	PnL        float64 `json:"pnl"`
	Volume     float64 `json:"volume"`
	TradeCount int     `json:"trade_count"`
}

type birdeyeTraderResponse struct {
	Data struct {
		Items []birdeyeTraderItem `json:"items"`
	} `json:"data"`
	Success bool `json:"success"`
}

// Valid period constants for Birdeye free-tier leaderboard.
const (
	PeriodToday = "today"
	Period1W    = "1W"
)

// AllPeriods lists every valid period to query for maximum coverage.
var AllPeriods = []string{PeriodToday, Period1W}

// BirdeyeCandidate is the discovery result for one wallet.
type BirdeyeCandidate struct {
	Address     string
	PnL         float64
	Volume      float64
	TradeCount  int
	PeriodCount int // how many leaderboard periods this wallet appeared in (1–2)
}

const pageSize = 10 // Birdeye free tier max per request

// TopTraders fetches up to `limit` top traders by paginating the Birdeye leaderboard.
// The free tier allows max 10 per request, so we paginate automatically.
func (b *BirdeyeClient) TopTraders(ctx context.Context, period string, limit int) ([]BirdeyeCandidate, error) {
	if b.apiKey == "" {
		return nil, fmt.Errorf("BIRDEYE_API_KEY not set")
	}

	var all []BirdeyeCandidate
	offset := 0

	for len(all) < limit {
		select {
		case <-ctx.Done():
			return all, ctx.Err()
		default:
		}

		batch, err := b.fetchPage(ctx, period, offset, pageSize)
		if err != nil {
			// Rate limited — wait and retry once
			if len(all) > 0 {
				time.Sleep(3 * time.Second)
				batch, err = b.fetchPage(ctx, period, offset, pageSize)
			}
			if err != nil {
				if len(all) > 0 {
					return all, nil // return partial results
				}
				return nil, err
			}
		}
		if len(batch) == 0 {
			break // end of results
		}

		all = append(all, batch...)
		offset += len(batch)

		if len(batch) < pageSize {
			break // last page
		}

		// Respect Birdeye rate limits between pages
		time.Sleep(1200 * time.Millisecond)
	}

	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func (b *BirdeyeClient) fetchPage(ctx context.Context, period string, offset, limit int) ([]BirdeyeCandidate, error) {
	url := fmt.Sprintf(
		"https://public-api.birdeye.so/trader/gainers-losers?type=%s&sort_by=PnL&sort_type=desc&offset=%d&limit=%d",
		period, offset, limit,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-KEY", b.apiKey)
	req.Header.Set("x-chain", "solana")
	req.Header.Set("accept", "application/json")

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("birdeye request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited (429)")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result birdeyeTraderResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	out := make([]BirdeyeCandidate, 0, len(result.Data.Items))
	for _, item := range result.Data.Items {
		out = append(out, BirdeyeCandidate{
			Address:    item.Address,
			PnL:        item.PnL,
			Volume:     item.Volume,
			TradeCount: item.TradeCount,
		})
	}
	return out, nil
}
