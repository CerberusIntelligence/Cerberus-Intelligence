package wallettracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"solana-trading-bot/config"
	"solana-trading-bot/types"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	log "github.com/sirupsen/logrus"
)

const persistFile = "tracked_wallets.json"

// Stablecoins and well-known program addresses to ignore as copy targets
var skipMints = map[string]bool{
	"So11111111111111111111111111111111111111112":   true, // SOL
	"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v": true, // USDC
	"Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB": true, // USDT
}

type Tracker struct {
	cfg      *config.Config
	client   *http.Client
	signalCh chan<- types.Signal
	mu       sync.RWMutex
	wallets  map[string]bool
}

func New(cfg *config.Config, signalCh chan<- types.Signal) *Tracker {
	t := &Tracker{
		cfg:      cfg,
		client:   &http.Client{Timeout: 8 * time.Second},
		signalCh: signalCh,
		wallets:  make(map[string]bool),
	}
	t.load()
	return t
}

func (t *Tracker) AddWallet(address string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.wallets[address] {
		t.wallets[address] = true
		log.WithField("wallet", address[:8]+"...").Info("Now tracking wallet")
		t.saveLocked()
	}
}

func (t *Tracker) WalletCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.wallets)
}

func (t *Tracker) GetWallets() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	addrs := make([]string, 0, len(t.wallets))
	for addr := range t.wallets {
		addrs = append(addrs, addr)
	}
	return addrs
}

// Start connects to Helius WebSocket and subscribes to all tracked wallets in real-time.
func (t *Tracker) Start(ctx context.Context) {
	log.WithField("wallets", t.WalletCount()).Info("Wallet tracker started (WebSocket real-time)")
	for {
		if err := t.runWebSocket(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.WithError(err).Warn("WebSocket disconnected — reconnecting in 5s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (t *Tracker) runWebSocket(ctx context.Context) error {
	wsURL := t.cfg.SolanaWSURL
	if wsURL == "" {
		wsURL = fmt.Sprintf("wss://mainnet.helius-rpc.com/?api-key=%s", t.cfg.HeliusAPIKey)
	}

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.CloseNow()
	log.Info("WebSocket connected to Helius")

	t.mu.RLock()
	wallets := make([]string, 0, len(t.wallets))
	for addr := range t.wallets {
		wallets = append(wallets, addr)
	}
	t.mu.RUnlock()

	subIDToWallet := make(map[int]string)
	for i, addr := range wallets {
		subID := i + 1
		sub := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      subID,
			"method":  "logsSubscribe",
			"params": []interface{}{
				map[string]interface{}{"mentions": []string{addr}},
				map[string]interface{}{"commitment": "processed"}, // processed = fastest notification
			},
		}
		if err := wsjson.Write(ctx, conn, sub); err != nil {
			return fmt.Errorf("subscribe: %w", err)
		}
		subIDToWallet[subID] = addr
	}

	confirmedSubs := make(map[int]string)

	for {
		var msg json.RawMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			return fmt.Errorf("read: %w", err)
		}

		var parsed map[string]json.RawMessage
		if err := json.Unmarshal(msg, &parsed); err != nil {
			continue
		}

		if idRaw, ok := parsed["id"]; ok && parsed["result"] != nil {
			var reqID int
			var subResult int
			if err := json.Unmarshal(idRaw, &reqID); err == nil {
				if err := json.Unmarshal(parsed["result"], &subResult); err == nil {
					if wallet, ok := subIDToWallet[reqID]; ok {
						confirmedSubs[subResult] = wallet
						log.WithFields(log.Fields{
							"wallet": wallet[:8] + "...",
							"sub":    subResult,
						}).Info("Wallet subscription confirmed")
					}
				}
			}
			continue
		}

		if methodRaw, ok := parsed["method"]; ok {
			var method string
			if err := json.Unmarshal(methodRaw, &method); err != nil || method != "logsNotification" {
				continue
			}

			var notif logsNotification
			if err := json.Unmarshal(msg, &notif); err != nil {
				continue
			}

			walletAddr := confirmedSubs[notif.Params.Subscription]
			if walletAddr == "" {
				continue
			}

			sig := notif.Params.Result.Value.Signature
			if sig == "" || notif.Params.Result.Value.Err != nil {
				continue
			}

			log.WithFields(log.Fields{
				"wallet": walletAddr[:8] + "...",
				"sig":    sig[:8] + "...",
			}).Info("Wallet transaction detected — fetching details")

			go t.processTx(ctx, walletAddr, sig)
		}
	}
}

// processTx fetches transaction details via Solana RPC and emits buy/sell signals.
// Uses fast retry instead of a fixed sleep — no Helius indexing delay needed.
func (t *Tracker) processTx(ctx context.Context, walletAddr, signature string) {
	var transfers []tokenTransfer

	// Try immediately, retry up to 5 times with 200ms between attempts
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(200 * time.Millisecond):
			}
		}

		txCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		tt, err := t.fetchTx(txCtx, walletAddr, signature)
		cancel()

		if err == nil && len(tt) > 0 {
			transfers = tt
			break
		}
	}

	if len(transfers) == 0 {
		return
	}

	emitted := make(map[string]bool)
	for _, tt := range transfers {
		if skipMints[tt.Mint] || tt.Mint == "" || emitted[tt.Mint] {
			continue
		}

		var isSell bool
		if tt.ToUserAccount == walletAddr {
			isSell = false
		} else if tt.FromUserAccount == walletAddr {
			isSell = true
		} else {
			continue
		}

		emitted[tt.Mint] = true

		if isSell {
			log.WithFields(log.Fields{
				"wallet": walletAddr[:8] + "...",
				"mint":   tt.Mint[:8] + "...",
			}).Info("Wallet sell detected — emitting copy sell signal")
		} else {
			log.WithFields(log.Fields{
				"wallet": walletAddr[:8] + "...",
				"mint":   tt.Mint[:8] + "...",
			}).Info("Wallet buy detected — emitting copy signal")
		}

		select {
		case t.signalCh <- types.Signal{
			Address:   tt.Mint,
			Source:    fmt.Sprintf("wallet:%s", walletAddr[:8]),
			Message:   "copy-trade",
			Timestamp: time.Now(),
			IsSell:    isSell,
			Price:     tt.PriceSOL,
		}:
		default:
		}
	}
}

// --- Solana RPC -----------------------------------------------------------

type tokenTransfer struct {
	FromUserAccount string
	ToUserAccount   string
	Mint            string
	PriceSOL        float64 // exact execution price from tx balance delta
}

type rpcTokenBalance struct {
	Mint          string `json:"mint"`
	Owner         string `json:"owner"`
	UITokenAmount struct {
		Amount   string `json:"amount"`
		Decimals int    `json:"decimals"`
	} `json:"uiTokenAmount"`
}

// fetchTx calls Solana RPC getTransaction, derives buy/sell transfers from token
// balance changes, and calculates exact execution price from SOL/token deltas.
func (t *Tracker) fetchTx(ctx context.Context, walletAddr, signature string) ([]tokenTransfer, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getTransaction",
		"params": []interface{}{
			signature,
			map[string]interface{}{
				"encoding":                       "jsonParsed",
				"commitment":                     "confirmed",
				"maxSupportedTransactionVersion": 0,
			},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", t.cfg.SolanaRPCURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Result *struct {
			Meta struct {
				Fee               int64             `json:"fee"`
				PreBalances       []int64           `json:"preBalances"`
				PostBalances      []int64           `json:"postBalances"`
				PreTokenBalances  []rpcTokenBalance `json:"preTokenBalances"`
				PostTokenBalances []rpcTokenBalance `json:"postTokenBalances"`
			} `json:"meta"`
			Transaction struct {
				Message struct {
					AccountKeys []struct {
						Pubkey string `json:"pubkey"`
					} `json:"accountKeys"`
				} `json:"message"`
			} `json:"transaction"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Result == nil {
		return nil, fmt.Errorf("tx not confirmed yet")
	}

	meta := result.Result.Meta
	keys := result.Result.Transaction.Message.AccountKeys

	// Find wallet's index in account keys for SOL balance lookup
	walletIdx := -1
	for i, k := range keys {
		if k.Pubkey == walletAddr {
			walletIdx = i
			break
		}
	}

	// Helper: token raw amount -> float64 applying decimals
	tokenFloat := func(raw string, decimals int) float64 {
		amt, _ := strconv.ParseInt(raw, 10, 64)
		if decimals == 0 {
			return float64(amt)
		}
		div := 1.0
		for i := 0; i < decimals; i++ {
			div *= 10
		}
		return float64(amt) / div
	}

	// Build pre-balance maps
	preTok := make(map[string]rpcTokenBalance)
	for _, b := range meta.PreTokenBalances {
		if b.Owner != "" && b.Mint != "" {
			preTok[b.Owner+":"+b.Mint] = b
		}
	}
	postTok := make(map[string]rpcTokenBalance)
	for _, b := range meta.PostTokenBalances {
		if b.Owner != "" && b.Mint != "" {
			postTok[b.Owner+":"+b.Mint] = b
		}
	}

	// SOL delta for the wallet (lamports -> SOL)
	walletSOLDelta := 0.0
	if walletIdx >= 0 && walletIdx < len(meta.PreBalances) && walletIdx < len(meta.PostBalances) {
		walletSOLDelta = float64(meta.PostBalances[walletIdx]-meta.PreBalances[walletIdx]) / 1e9
	}

	var transfers []tokenTransfer

	// Tokens that increased = received (buy)
	for _, b := range meta.PostTokenBalances {
		if b.Owner == "" || b.Mint == "" {
			continue
		}
		key := b.Owner + ":" + b.Mint
		pre := preTok[key]
		postAmt := tokenFloat(b.UITokenAmount.Amount, b.UITokenAmount.Decimals)
		preAmt := tokenFloat(pre.UITokenAmount.Amount, pre.UITokenAmount.Decimals)
		tokenDelta := postAmt - preAmt
		if tokenDelta <= 0 {
			continue
		}
		// Price: SOL spent / tokens received (SOL delta is negative for buyer)
		price := 0.0
		if b.Owner == walletAddr && walletSOLDelta < 0 && tokenDelta > 0 {
			price = (-walletSOLDelta - float64(meta.Fee)/1e9) / tokenDelta
		}
		transfers = append(transfers, tokenTransfer{
			ToUserAccount: b.Owner,
			Mint:          b.Mint,
			PriceSOL:      price,
		})
	}

	// Tokens that decreased or vanished = sent (sell)
	for _, b := range meta.PreTokenBalances {
		if b.Owner == "" || b.Mint == "" {
			continue
		}
		key := b.Owner + ":" + b.Mint
		post := postTok[key]
		preAmt := tokenFloat(b.UITokenAmount.Amount, b.UITokenAmount.Decimals)
		postAmt := tokenFloat(post.UITokenAmount.Amount, post.UITokenAmount.Decimals)
		tokenDelta := preAmt - postAmt
		if tokenDelta <= 0 {
			continue
		}
		// Price: SOL received / tokens sold (SOL delta is positive for seller)
		price := 0.0
		if b.Owner == walletAddr && walletSOLDelta > 0 && tokenDelta > 0 {
			price = (walletSOLDelta + float64(meta.Fee)/1e9) / tokenDelta
		}
		transfers = append(transfers, tokenTransfer{
			FromUserAccount: b.Owner,
			Mint:            b.Mint,
			PriceSOL:        price,
		})
	}

	return transfers, nil
}

// --- WebSocket message types ----------------------------------------------

type logsNotification struct {
	Params struct {
		Subscription int `json:"subscription"`
		Result       struct {
			Value struct {
				Signature string      `json:"signature"`
				Err       interface{} `json:"err"`
				Logs      []string    `json:"logs"`
			} `json:"value"`
		} `json:"result"`
	} `json:"params"`
}

// --- Persistence ----------------------------------------------------------

func (t *Tracker) load() {
	data, err := os.ReadFile(persistFile)
	if err != nil {
		return
	}
	var boolFmt map[string]bool
	if err := json.Unmarshal(data, &boolFmt); err == nil {
		t.wallets = boolFmt
	} else {
		var strFmt map[string]string
		if err := json.Unmarshal(data, &strFmt); err == nil {
			for addr := range strFmt {
				t.wallets[addr] = true
			}
		} else {
			return
		}
	}
	log.WithField("count", len(t.wallets)).Info("Loaded tracked wallets")
}

func (t *Tracker) saveLocked() {
	data, err := json.MarshalIndent(t.wallets, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(persistFile, data, 0644)
}
