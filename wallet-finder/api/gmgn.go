package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const gmgnBase = "https://gmgn.ai"

// GMGNClient scrapes GMGN.ai's internal leaderboard APIs.
type GMGNClient struct {
	http *http.Client
}

func NewGMGN() *GMGNClient {
	return &GMGNClient{
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// gmgnRawWallet is the raw shape GMGN returns (profits as strings).
type gmgnRawWallet struct {
	Address           string   `json:"address"`
	LastActive        int64    `json:"last_active"`
	RealizedProfit7d  string   `json:"realized_profit_7d"`
	RealizedProfit30d string   `json:"realized_profit_30d"`
	Buy               int      `json:"buy"`
	Buy7d             int      `json:"buy_7d"`
	Buy30d            int      `json:"buy_30d"`
	Sell              int      `json:"sell"`
	Sell7d            int      `json:"sell_7d"`
	Sell30d           int      `json:"sell_30d"`
	Winrate7d         float64  `json:"winrate_7d"`
	Winrate30d        float64  `json:"winrate_30d"`
	Tags              []string `json:"tags"`
}

// GMGNWallet is the parsed, usable wallet entry from GMGN's rank API.
type GMGNWallet struct {
	Address           string
	LastActiveTime    int64
	RealizedProfit7d  float64
	RealizedProfit30d float64
	Buy7d             int
	Buy30d            int
	Sell7d            int
	Sell30d           int
	Buy               int
	Sell              int
	Winrate7d         float64
	Winrate30d        float64
	Tags              []string
}

type gmgnRankResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Rank []gmgnRawWallet `json:"rank"`
	} `json:"data"`
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func convertRaw(r gmgnRawWallet) GMGNWallet {
	return GMGNWallet{
		Address:           r.Address,
		LastActiveTime:    r.LastActive,
		RealizedProfit7d:  parseFloat(r.RealizedProfit7d),
		RealizedProfit30d: parseFloat(r.RealizedProfit30d),
		Buy7d:             r.Buy7d,
		Buy30d:            r.Buy30d,
		Sell7d:            r.Sell7d,
		Sell30d:           r.Sell30d,
		Buy:               r.Buy,
		Sell:              r.Sell,
		Winrate7d:         r.Winrate7d,
		Winrate30d:        r.Winrate30d,
		Tags:              r.Tags,
	}
}

// Periods supported by GMGN leaderboard.
var GMGNPeriods = []string{"7d", "30d"}

func (g *GMGNClient) newRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://gmgn.ai/sol/wallets")
	req.Header.Set("Origin", "https://gmgn.ai")
	return req, nil
}

// TopWalletsPaged fetches multiple pages to get up to `total` wallets for a period.
// period: "7d" or "30d", orderby: "pnl" or "winrate"
func (g *GMGNClient) TopWalletsPaged(ctx context.Context, period string, orderby string, total int) ([]GMGNWallet, error) {
	pageSize := 100
	if total < pageSize {
		pageSize = total
	}

	seen := make(map[string]bool)
	var all []GMGNWallet

	for offset := 0; len(all) < total; offset += pageSize {
		select {
		case <-ctx.Done():
			return all, nil
		default:
		}

		url := fmt.Sprintf(
			"%s/defi/quotation/v1/rank/sol/wallets/%s?orderby=%s&direction=desc&limit=%d&offset=%d",
			gmgnBase, period, orderby, pageSize, offset,
		)

		req, err := g.newRequest(ctx, url)
		if err != nil {
			return all, err
		}

		resp, err := g.http.Do(req)
		if err != nil {
			return all, fmt.Errorf("gmgn page %d failed: %w", offset/pageSize+1, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			return all, fmt.Errorf("gmgn HTTP %d at offset %d", resp.StatusCode, offset)
		}

		var result gmgnRankResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return all, fmt.Errorf("gmgn decode error at offset %d: %w", offset, err)
		}

		if len(result.Data.Rank) == 0 {
			break
		}

		for _, raw := range result.Data.Rank {
			if raw.Address != "" && !seen[raw.Address] {
				seen[raw.Address] = true
				all = append(all, convertRaw(raw))
			}
		}

		if len(result.Data.Rank) < pageSize {
			break
		}

		time.Sleep(600 * time.Millisecond)
	}

	return all, nil
}

// SmartMoneyWallets fetches wallets tagged as "smart_degen" on GMGN.
func (g *GMGNClient) SmartMoneyWallets(ctx context.Context, period string, limit int) ([]GMGNWallet, error) {
	url := fmt.Sprintf(
		"%s/defi/quotation/v1/rank/sol/wallets/%s?orderby=pnl&direction=desc&limit=%d&tag=smart_degen",
		gmgnBase, period, limit,
	)

	req, err := g.newRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gmgn smart money request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gmgn smart money HTTP %d", resp.StatusCode)
	}

	var result gmgnRankResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("gmgn smart money decode error: %w", err)
	}

	var out []GMGNWallet
	for _, raw := range result.Data.Rank {
		out = append(out, convertRaw(raw))
	}
	return out, nil
}
