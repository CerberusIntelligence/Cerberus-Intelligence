package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const dexBase = "https://api.dexscreener.com"

type DexPair struct {
	PairAddress   string       `json:"pairAddress"`
	ChainID       string       `json:"chainId"`
	BaseToken     DexToken     `json:"baseToken"`
	QuoteToken    DexToken     `json:"quoteToken"`
	PriceChange   DexChange    `json:"priceChange"`
	Volume        DexVolume    `json:"volume"`
	Liquidity     DexLiquidity `json:"liquidity"`
	PairCreatedAt int64        `json:"pairCreatedAt"`
}

type DexToken struct {
	Address string `json:"address"`
	Symbol  string `json:"symbol"`
	Name    string `json:"name"`
}

type DexChange struct {
	H1  float64 `json:"h1"`
	H6  float64 `json:"h6"`
	H24 float64 `json:"h24"`
}

type DexVolume struct {
	H24 float64 `json:"h24"`
}

type DexLiquidity struct {
	USD float64 `json:"usd"`
}

type DexClient struct {
	http *http.Client
}

func NewDexscreener() *DexClient {
	return &DexClient{http: &http.Client{Timeout: 20 * time.Second}}
}

// PumpedToken is a token that has pumped significantly.
type PumpedToken struct {
	PairAddress    string
	TokenMint      string
	Symbol         string
	PriceChange24h float64
	VolumeUSD      float64
	LiquidityUSD   float64
}

// FindPumpedTokens searches DexScreener for Solana tokens that have pumped significantly.
func (d *DexClient) FindPumpedTokens(ctx context.Context, minChange float64, minVolumeUSD float64) ([]PumpedToken, error) {
	queries := []string{"pump", "sol", "pepe", "bonk", "meme", "dog", "cat", "ai", "trump", "fart"}
	seen := make(map[string]bool)
	var results []PumpedToken

	const solMint = "So11111111111111111111111111111111111111112"

	for _, q := range queries {
		select {
		case <-ctx.Done():
			return results, nil
		default:
		}

		pairs, err := d.search(ctx, q)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for _, p := range pairs {
			if p.ChainID != "solana" {
				continue
			}
			if p.QuoteToken.Address != solMint {
				continue
			}
			if p.PriceChange.H24 < minChange {
				continue
			}
			if p.Volume.H24 < minVolumeUSD {
				continue
			}
			if p.Liquidity.USD < 5000 {
				continue
			}
			if seen[p.PairAddress] {
				continue
			}
			seen[p.PairAddress] = true
			results = append(results, PumpedToken{
				PairAddress:    p.PairAddress,
				TokenMint:      p.BaseToken.Address,
				Symbol:         p.BaseToken.Symbol,
				PriceChange24h: p.PriceChange.H24,
				VolumeUSD:      p.Volume.H24,
				LiquidityUSD:   p.Liquidity.USD,
			})
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Also try token-boosts endpoint for trending tokens
	boosted, _ := d.fetchBoosted(ctx, solMint, minVolumeUSD)
	for _, b := range boosted {
		if !seen[b.PairAddress] {
			seen[b.PairAddress] = true
			results = append(results, b)
		}
	}

	return results, nil
}

func (d *DexClient) fetchBoosted(ctx context.Context, solMint string, minVolume float64) ([]PumpedToken, error) {
	url := fmt.Sprintf("%s/token-boosts/latest/v1", dexBase)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var items []struct {
		TokenAddress string `json:"tokenAddress"`
		ChainId      string `json:"chainId"`
		Links        []struct {
			Label string `json:"label"`
			URL   string `json:"url"`
		} `json:"links"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}

	var results []PumpedToken
	for _, item := range items {
		if item.ChainId != "solana" {
			continue
		}
		// Look up this token's pairs
		pairs, err := d.fetchTokenPairs(ctx, item.TokenAddress)
		if err != nil {
			continue
		}
		for _, p := range pairs {
			if p.QuoteToken.Address == solMint && p.Volume.H24 >= minVolume {
				results = append(results, PumpedToken{
					PairAddress:    p.PairAddress,
					TokenMint:      item.TokenAddress,
					Symbol:         p.BaseToken.Symbol,
					PriceChange24h: p.PriceChange.H24,
					VolumeUSD:      p.Volume.H24,
					LiquidityUSD:   p.Liquidity.USD,
				})
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return results, nil
}

func (d *DexClient) fetchTokenPairs(ctx context.Context, tokenAddr string) ([]DexPair, error) {
	url := fmt.Sprintf("%s/latest/dex/tokens/%s", dexBase, tokenAddr)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Pairs []DexPair `json:"pairs"`
	}
	_ = json.Unmarshal(body, &result)
	return result.Pairs, nil
}

func (d *DexClient) search(ctx context.Context, query string) ([]DexPair, error) {
	url := fmt.Sprintf("%s/latest/dex/search?q=%s", dexBase, query)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Pairs []DexPair `json:"pairs"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.Pairs, nil
}
