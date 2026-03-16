package api

import (
	"context"
	"crypto/tls"
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
	// Force HTTP/1.1 by disabling HTTP/2 — Cloudflare treats Go's HTTP/2
	// TLS fingerprint as a bot. HTTP/1.1 matches curl's default behavior.
	transport := &http.Transport{
		TLSNextProto: make(map[string]func(string, *tls.Conn) http.RoundTripper),
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	return &GMGNClient{
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// gmgnRawWallet is the raw shape GMGN returns (profits as strings).
type gmgnRawWallet struct {
	Address              string   `json:"address"`
	LastActive           int64    `json:"last_active"`
	RealizedProfit7d     string   `json:"realized_profit_7d"`
	RealizedProfit30d    string   `json:"realized_profit_30d"`
	Buy                  int      `json:"buy"`
	Buy7d                int      `json:"buy_7d"`
	Buy30d               int      `json:"buy_30d"`
	Sell                 int      `json:"sell"`
	Sell7d               int      `json:"sell_7d"`
	Sell30d              int      `json:"sell_30d"`
	Winrate7d            float64  `json:"winrate_7d"`
	Winrate30d           float64  `json:"winrate_30d"`
	AvgHoldingPeriod7d   float64  `json:"avg_holding_period_7d"`  // seconds
	AvgHoldingPeriod30d  float64  `json:"avg_holding_period_30d"` // seconds
	Tags                 []string `json:"tags"`
}

// GMGNWallet is the parsed, usable wallet entry from GMGN's rank API.
type GMGNWallet struct {
	Address              string
	LastActiveTime       int64
	RealizedProfit7d     float64
	RealizedProfit30d    float64
	Buy7d                int
	Buy30d               int
	Sell7d               int
	Sell30d              int
	Buy                  int
	Sell                 int
	Winrate7d            float64
	Winrate30d           float64
	AvgHoldingPeriod7d   float64 // seconds
	AvgHoldingPeriod30d  float64 // seconds
	Tags                 []string
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
		Winrate7d:           r.Winrate7d,
		Winrate30d:          r.Winrate30d,
		AvgHoldingPeriod7d:  r.AvgHoldingPeriod7d,
		AvgHoldingPeriod30d: r.AvgHoldingPeriod30d,
		Tags:                r.Tags,
	}
}

// Periods supported by GMGN leaderboard.
var GMGNPeriods = []string{"7d", "30d"}

func (g *GMGNClient) newRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Full Chrome 122 header set — order matters for Cloudflare fingerprinting
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// Do NOT set Accept-Encoding manually — Go's transport adds it and handles
	// decompression automatically. Manual setting disables auto-decompression.
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "https://gmgn.ai/sol/wallets")
	req.Header.Set("Origin", "https://gmgn.ai")
	req.Header.Set("sec-ch-ua", `"Not(A:Brand";v="99", "Google Chrome";v="122", "Chromium";v="122"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	return req, nil
}

func (g *GMGNClient) doRequest(req *http.Request) ([]byte, int, error) {
	resp, err := g.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

// TopWalletsPaged fetches up to maxPages pages of wallets for a period.
// period: "7d" or "30d", orderby: "pnl" or "winrate"
func (g *GMGNClient) TopWalletsPaged(ctx context.Context, period string, orderby string, total int) ([]GMGNWallet, error) {
	const pageSize = 100
	const maxPages = 5 // GMGN cycles duplicates after ~5 pages

	seen := make(map[string]bool)
	var all []GMGNWallet

	for page := 0; page < maxPages; page++ {
		select {
		case <-ctx.Done():
			return all, nil
		default:
		}

		offset := page * pageSize
		url := fmt.Sprintf(
			"%s/defi/quotation/v1/rank/sol/wallets/%s?orderby=%s&direction=desc&limit=%d&offset=%d",
			gmgnBase, period, orderby, pageSize, offset,
		)

		req, err := g.newRequest(ctx, url)
		if err != nil {
			return all, err
		}

		fmt.Printf("    page %d...", page+1)
		body, status, err := g.doRequest(req)
		if err != nil {
			fmt.Println(" err")
			return all, fmt.Errorf("gmgn page %d failed: %w", page+1, err)
		}
		if status != 200 {
			fmt.Println(" blocked")
			return all, fmt.Errorf("gmgn HTTP %d at offset %d", status, offset)
		}

		var result gmgnRankResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return all, fmt.Errorf("gmgn decode error at offset %d: %w", offset, err)
		}

		if len(result.Data.Rank) == 0 {
			fmt.Println(" done")
			break
		}

		added := 0
		for _, raw := range result.Data.Rank {
			if raw.Address != "" && !seen[raw.Address] {
				seen[raw.Address] = true
				all = append(all, convertRaw(raw))
				added++
			}
		}
		fmt.Printf(" %d new (%d total)\n", added, len(all))

		// Stop if GMGN is cycling duplicates
		if added == 0 {
			break
		}
		if len(result.Data.Rank) < pageSize {
			break
		}

		time.Sleep(800 * time.Millisecond)
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

	body, status, err := g.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("gmgn smart money request failed: %w", err)
	}
	if status != 200 {
		return nil, fmt.Errorf("gmgn smart money HTTP %d", status)
	}

	var result gmgnRankResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("gmgn smart money decode error: %w", err)
	}

	var out []GMGNWallet
	for _, raw := range result.Data.Rank {
		out = append(out, convertRaw(raw))
	}
	return out, nil
}
