package scanner

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"solana-trading-bot/config"
	"solana-trading-bot/types"

	log "github.com/sirupsen/logrus"
)

const solMint = "So11111111111111111111111111111111111111112"

type Scanner struct {
	cfg    *config.Config
	client *http.Client
}

func New(cfg *config.Config) *Scanner {
	return &Scanner{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type ValidationResult struct {
	Valid  bool
	Info   *types.TokenInfo
	Reason string
}

// Validate checks a token against all safety filters.
func (s *Scanner) Validate(ctx context.Context, address string) *ValidationResult {
	// 1. Fetch market data from DexScreener
	info, err := s.fetchDexScreener(ctx, address)
	if err != nil {
		log.WithError(err).WithField("address", address[:8]).Warn("DexScreener fetch failed")
		return &ValidationResult{Valid: false, Reason: "dexscreener fetch failed: " + err.Error()}
	}
	if info == nil {
		return &ValidationResult{Valid: false, Reason: "token not found on any DEX"}
	}

	// 2. Minimum liquidity check
	if info.LiquidityUSD < s.cfg.MinLiquidityUSD {
		return &ValidationResult{
			Valid:  false,
			Info:   info,
			Reason: fmt.Sprintf("liquidity $%.0f < min $%.0f", info.LiquidityUSD, s.cfg.MinLiquidityUSD),
		}
	}

	// 3. Mint / freeze authority check via Helius RPC
	mintRevoked, freezeRevoked, err := s.checkAuthorities(ctx, address)
	if err != nil {
		log.WithError(err).WithField("address", address[:8]).Debug("Authority check failed, continuing")
	} else {
		info.MintRevoked = mintRevoked
		info.FreezeRevoked = freezeRevoked
	}

	if s.cfg.RequireMintRevoked && !info.MintRevoked {
		return &ValidationResult{Valid: false, Info: info, Reason: "mint authority not revoked"}
	}
	if s.cfg.RequireFreezeRevoked && !info.FreezeRevoked {
		return &ValidationResult{Valid: false, Info: info, Reason: "freeze authority not revoked"}
	}

	return &ValidationResult{Valid: true, Info: info}
}

// --- DexScreener ----------------------------------------------------------

type dexPair struct {
	ChainID  string `json:"chainId"`
	DexID    string `json:"dexId"`
	BaseToken struct {
		Address string `json:"address"`
		Name    string `json:"name"`
		Symbol  string `json:"symbol"`
	} `json:"baseToken"`
	PriceNative string `json:"priceNative"`
	PriceUsd    string `json:"priceUsd"`
	Volume      struct {
		H24 float64 `json:"h24"`
	} `json:"volume"`
	PriceChange struct {
		M5 float64 `json:"m5"`
	} `json:"priceChange"`
	Liquidity struct {
		USD   float64 `json:"usd"`
		Quote float64 `json:"quote"` // SOL amount in pool
	} `json:"liquidity"`
	PairCreatedAt int64 `json:"pairCreatedAt"`
}

type dexScreenerResp struct {
	Pairs []dexPair `json:"pairs"`
}

// FetchMarketData is the public version of fetchDexScreener.
func (s *Scanner) FetchMarketData(ctx context.Context, address string) (*types.TokenInfo, error) {
	return s.fetchDexScreener(ctx, address)
}

func (s *Scanner) fetchDexScreener(ctx context.Context, address string) (*types.TokenInfo, error) {
	url := "https://api.dexscreener.com/latest/dex/tokens/" + address

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data dexScreenerResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if len(data.Pairs) == 0 {
		return nil, nil
	}

	// Pick the most liquid Solana pair
	var best *dexPair
	for i := range data.Pairs {
		p := &data.Pairs[i]
		if p.ChainID != "solana" {
			continue
		}
		if best == nil || p.Liquidity.USD > best.Liquidity.USD {
			best = p
		}
	}
	if best == nil {
		return nil, nil
	}

	info := &types.TokenInfo{
		Address:       address,
		Symbol:        best.BaseToken.Symbol,
		Name:          best.BaseToken.Name,
		PriceSOL:      parseFloat(best.PriceNative),
		PriceUSD:      parseFloat(best.PriceUsd),
		LiquidityUSD:  best.Liquidity.USD,
		LiquiditySOL:  best.Liquidity.Quote,
		Volume24h:     best.Volume.H24,
		PriceChange5m: best.PriceChange.M5,
	}
	if best.PairCreatedAt > 0 {
		info.CreatedAt = time.Unix(best.PairCreatedAt/1000, 0)
	}

	return info, nil
}

// --- On-chain authority checks --------------------------------------------

func (s *Scanner) checkAuthorities(ctx context.Context, mintAddress string) (mintRevoked, freezeRevoked bool, err error) {
	type rpcReq struct {
		JSONRPC string        `json:"jsonrpc"`
		ID      string        `json:"id"`
		Method  string        `json:"method"`
		Params  []interface{} `json:"params"`
	}

	body, err := json.Marshal(rpcReq{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "getAccountInfo",
		Params:  []interface{}{mintAddress, map[string]string{"encoding": "base64"}},
	})
	if err != nil {
		return false, false, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.SolanaRPCURL, bytes.NewReader(body))
	if err != nil {
		return false, false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()

	var result struct {
		Result struct {
			Value struct {
				Data []interface{} `json:"data"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, false, err
	}

	if len(result.Result.Value.Data) < 1 {
		return false, false, fmt.Errorf("no account data")
	}

	dataStr, ok := result.Result.Value.Data[0].(string)
	if !ok {
		return false, false, fmt.Errorf("unexpected data format")
	}

	raw, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return false, false, err
	}

	// SPL Token Mint layout: mintAuthorityOption at byte 0 (u32), freezeAuthorityOption at byte 46 (u32)
	if len(raw) < 82 {
		return false, false, fmt.Errorf("account data too short (%d bytes)", len(raw))
	}

	mintRevoked = binary.LittleEndian.Uint32(raw[0:4]) == 0
	freezeRevoked = binary.LittleEndian.Uint32(raw[46:50]) == 0
	return mintRevoked, freezeRevoked, nil
}

// --- Price fetching -------------------------------------------------------

// GetPrice fetches current price in SOL.
// Tries Jupiter first (fast), falls back to DexScreener for pump.fun bonding curve tokens.
func (s *Scanner) GetPrice(ctx context.Context, address string) (float64, error) {
	prices, err := s.GetPrices(ctx, []string{address})
	if err == nil && prices[address] > 0 {
		return prices[address], nil
	}

	// Jupiter has no price — try DexScreener (handles pump.fun bonding curve tokens)
	info, err2 := s.fetchDexScreener(ctx, address)
	if err2 == nil && info != nil && info.PriceSOL > 0 {
		return info.PriceSOL, nil
	}

	return 0, fmt.Errorf("no price available from any source")
}

// GetPrices fetches current prices in SOL for multiple tokens.
// Tries Jupiter first; if it fails falls back to DexScreener per-token in parallel.
func (s *Scanner) GetPrices(ctx context.Context, addresses []string) (map[string]float64, error) {
	if len(addresses) == 0 {
		return map[string]float64{}, nil
	}

	// Try Jupiter batch first
	prices, err := s.jupiterPrices(ctx, addresses)
	if err == nil && len(prices) > 0 {
		return prices, nil
	}

	// Jupiter failed — fetch DexScreener for each address in parallel
	log.Debug("Jupiter unavailable, falling back to DexScreener for prices")
	return s.dexScreenerPrices(ctx, addresses)
}

func (s *Scanner) jupiterPrices(ctx context.Context, addresses []string) (map[string]float64, error) {
	ids := strings.Join(addresses, ",")
	url := fmt.Sprintf("https://price.jup.ag/v6/price?ids=%s&vsToken=%s", ids, solMint)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data map[string]struct {
			Price float64 `json:"price"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	prices := make(map[string]float64, len(result.Data))
	for addr, d := range result.Data {
		prices[addr] = d.Price
	}
	return prices, nil
}

func (s *Scanner) dexScreenerPrices(ctx context.Context, addresses []string) (map[string]float64, error) {
	type result struct {
		addr  string
		price float64
	}

	ch := make(chan result, len(addresses))
	for _, addr := range addresses {
		addr := addr
		go func() {
			info, err := s.fetchDexScreener(ctx, addr)
			if err == nil && info != nil {
				ch <- result{addr, info.PriceSOL}
			} else {
				ch <- result{addr, 0}
			}
		}()
	}

	prices := make(map[string]float64, len(addresses))
	for range addresses {
		r := <-ch
		if r.price > 0 {
			prices[r.addr] = r.price
		}
	}
	return prices, nil
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
