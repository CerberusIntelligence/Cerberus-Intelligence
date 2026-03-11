package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"solana-trading-bot/config"
	"solana-trading-bot/types"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/sirupsen/logrus"
)

const (
	pollInterval    = 15 * time.Second // How often to poll each wallet for new txs
	maxTracked      = 10               // Max wallets to copy trade at once
	minWinRate      = 0.60             // Minimum win rate to keep tracking a wallet
	minTrades       = 5                // Minimum trades before evaluating a wallet
)

// WalletTracker monitors wallets for copy trading signals
type WalletTracker struct {
	cfg          *config.Config
	log          *logrus.Logger
	rpcClient    *rpc.Client
	wsClient     *ws.Client
	activityChan chan *types.WalletActivity
	walletStats  map[string]*WalletStats
	lastSig      map[string]string // last processed tx sig per wallet
	mu           sync.RWMutex
	httpClient   *http.Client
	ctx          context.Context
	cancel       context.CancelFunc

	// Notification callback — set by engine to send Telegram messages
	OnWalletAdded   func(addr, reason string)
	OnWalletRemoved func(addr, reason string)
}

// WalletStats tracks wallet performance
type WalletStats struct {
	Address       string
	TotalTrades   int
	WinningTrades int
	TotalPnL      float64
	WinRate       float64
	LastActivity  time.Time
	AddedAt       time.Time
	Tokens        map[string]float64 // token -> pnl
}

// NewWalletTracker creates a new wallet tracker
func NewWalletTracker(cfg *config.Config, log *logrus.Logger) *WalletTracker {
	ctx, cancel := context.WithCancel(context.Background())
	return &WalletTracker{
		cfg:          cfg,
		log:          log,
		rpcClient:    rpc.New(cfg.SolanaRPCURL),
		activityChan: make(chan *types.WalletActivity, 100),
		walletStats:  make(map[string]*WalletStats),
		lastSig:      make(map[string]string),
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Activities returns channel for wallet activity
func (w *WalletTracker) Activities() <-chan *types.WalletActivity {
	return w.activityChan
}

// Start begins monitoring tracked wallets
func (w *WalletTracker) Start(ctx context.Context) error {
	if !w.cfg.WalletTrackingEnabled {
		w.log.Info("Wallet tracking disabled")
		return nil
	}

	// Try to connect WebSocket (best effort — polling is the reliable path)
	wsClient, err := ws.Connect(ctx, w.cfg.SolanaWSURL)
	if err != nil {
		w.log.WithError(err).Warn("WebSocket unavailable — using polling only")
	} else {
		w.wsClient = wsClient
	}

	// Subscribe to any pre-configured wallets
	for _, wallet := range w.cfg.TrackedWallets {
		w.subscribeWallet(ctx, wallet)
	}

	// Polling loop — reliable fallback that catches everything WebSocket misses
	go w.pollLoop(ctx)

	w.log.WithField("wallets", len(w.cfg.TrackedWallets)).Info("Wallet tracker started")
	return nil
}

// pollLoop polls every wallet every 15 seconds for new transactions
func (w *WalletTracker) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.mu.RLock()
			wallets := make([]string, len(w.cfg.TrackedWallets))
			copy(wallets, w.cfg.TrackedWallets)
			w.mu.RUnlock()

			for _, wallet := range wallets {
				go w.pollWallet(ctx, wallet)
			}
		}
	}
}

// pollWallet checks for new transactions on a single wallet
func (w *WalletTracker) pollWallet(ctx context.Context, walletAddr string) {
	pubkey, err := solana.PublicKeyFromBase58(walletAddr)
	if err != nil {
		return
	}

	w.mu.RLock()
	last := w.lastSig[walletAddr]
	w.mu.RUnlock()

	opts := rpc.GetSignaturesForAddressOpts{
		Limit:      intPtr(10),
		Commitment: rpc.CommitmentConfirmed,
	}
	if last != "" {
		sig, err := solana.SignatureFromBase58(last)
		if err == nil {
			opts.Until = sig
		}
	}

	sigs, err := w.rpcClient.GetSignaturesForAddressWithOpts(ctx, pubkey, &opts)
	if err != nil || len(sigs) == 0 {
		return
	}

	// Update lastSig — always store the most recent
	w.mu.Lock()
	w.lastSig[walletAddr] = sigs[0].Signature.String()
	w.mu.Unlock()

	// Process in reverse order (oldest first)
	for i := len(sigs) - 1; i >= 0; i-- {
		sig := sigs[i]
		if sig.Signature.String() == last {
			continue
		}
		activity := w.parseTransaction(ctx, walletAddr, sig.Signature.String())
		if activity == nil {
			continue
		}
		select {
		case w.activityChan <- activity:
			w.log.WithFields(logrus.Fields{
				"wallet": walletAddr[:8] + "...",
				"action": activity.Action,
				"token":  activity.TokenAddress[:8] + "...",
				"sol":    fmt.Sprintf("%.3f", activity.AmountSOL),
			}).Info("Copy-trade signal detected (poll)")
		default:
			w.log.Warn("Activity channel full")
		}
	}
}

// subscribeWallet adds a real-time WebSocket subscription for instant detection
func (w *WalletTracker) subscribeWallet(ctx context.Context, walletAddr string) {
	if w.wsClient == nil {
		return
	}

	pubkey, err := solana.PublicKeyFromBase58(walletAddr)
	if err != nil {
		return
	}

	sub, err := w.wsClient.AccountSubscribe(pubkey, rpc.CommitmentConfirmed)
	if err != nil {
		w.log.WithError(err).WithField("wallet", walletAddr[:8]+"...").Warn("WS subscribe failed")
		return
	}

	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-sub.Response():
				if !ok {
					return
				}
				// Account changed — run an immediate poll to get the tx
				go w.pollWallet(ctx, walletAddr)
			case err := <-sub.Err():
				w.log.WithError(err).WithField("wallet", walletAddr[:8]+"...").Warn("WS subscription error")
				return
			}
		}
	}()

	w.log.WithField("wallet", walletAddr[:8]+"...").Info("WS subscribed to wallet")
}

// AddWallet adds a wallet and immediately starts monitoring it
func (w *WalletTracker) AddWallet(walletAddr string) bool {
	if !isValidSolanaAddress(walletAddr) {
		return false
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Check duplicate
	for _, existing := range w.cfg.TrackedWallets {
		if existing == walletAddr {
			return false
		}
	}

	// Cap at maxTracked
	if len(w.cfg.TrackedWallets) >= maxTracked {
		return false
	}

	w.cfg.TrackedWallets = append(w.cfg.TrackedWallets, walletAddr)
	w.walletStats[walletAddr] = &WalletStats{
		Address: walletAddr,
		AddedAt: time.Now(),
		Tokens:  make(map[string]float64),
	}

	w.log.WithField("wallet", walletAddr[:8]+"...").Info("Wallet added to copy trading")

	// Subscribe immediately (uses stored context)
	go w.subscribeWallet(w.ctx, walletAddr)

	if w.OnWalletAdded != nil {
		go w.OnWalletAdded(walletAddr, "manual")
	}
	return true
}

// RemoveWallet removes a wallet from tracking
func (w *WalletTracker) RemoveWallet(walletAddr string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for i, existing := range w.cfg.TrackedWallets {
		if existing == walletAddr {
			w.cfg.TrackedWallets = append(w.cfg.TrackedWallets[:i], w.cfg.TrackedWallets[i+1:]...)
			delete(w.walletStats, walletAddr)
			delete(w.lastSig, walletAddr)
			w.log.WithField("wallet", walletAddr[:8]+"...").Info("Wallet removed from copy trading")
			if w.OnWalletRemoved != nil {
				go w.OnWalletRemoved(walletAddr, "manual")
			}
			return
		}
	}
}

// AutoRotate replaces wallets below minWinRate with candidates from a new list.
// Called by the engine after each smart wallet scan.
func (w *WalletTracker) AutoRotate(candidates []string) (added, removed []string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Drop any wallet with enough trades but poor win rate
	keep := make([]string, 0, len(w.cfg.TrackedWallets))
	for _, addr := range w.cfg.TrackedWallets {
		stats := w.walletStats[addr]
		if stats != nil && stats.TotalTrades >= minTrades && stats.WinRate < minWinRate {
			w.log.WithFields(logrus.Fields{
				"wallet":  addr[:8] + "...",
				"winRate": fmt.Sprintf("%.0f%%", stats.WinRate*100),
				"trades":  stats.TotalTrades,
			}).Info("Rotating out underperforming wallet")
			removed = append(removed, addr)
			delete(w.walletStats, addr)
			delete(w.lastSig, addr)
			if w.OnWalletRemoved != nil {
				go w.OnWalletRemoved(addr, fmt.Sprintf("win rate %.0f%% below %.0f%% threshold", stats.WinRate*100, minWinRate*100))
			}
		} else {
			keep = append(keep, addr)
		}
	}
	w.cfg.TrackedWallets = keep

	// Add new candidates up to maxTracked
	existing := make(map[string]bool)
	for _, addr := range w.cfg.TrackedWallets {
		existing[addr] = true
	}

	for _, candidate := range candidates {
		if len(w.cfg.TrackedWallets) >= maxTracked {
			break
		}
		if existing[candidate] {
			continue
		}
		w.cfg.TrackedWallets = append(w.cfg.TrackedWallets, candidate)
		w.walletStats[candidate] = &WalletStats{
			Address: candidate,
			AddedAt: time.Now(),
			Tokens:  make(map[string]float64),
		}
		added = append(added, candidate)
		go w.subscribeWallet(w.ctx, candidate)
		if w.OnWalletAdded != nil {
			go w.OnWalletAdded(candidate, "auto-scan")
		}
	}

	return added, removed
}

// RecordCopyTradeResult updates win/loss stats for a wallet after a copy trade closes
func (w *WalletTracker) RecordCopyTradeResult(walletAddr string, isWin bool, pnl float64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	stats, ok := w.walletStats[walletAddr]
	if !ok {
		return
	}

	stats.TotalTrades++
	if isWin {
		stats.WinningTrades++
	}
	stats.TotalPnL += pnl
	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.WinningTrades) / float64(stats.TotalTrades)
	}
}

// GetTrackedWallets returns a snapshot of current wallets and their stats
func (w *WalletTracker) GetTrackedWallets() []*WalletStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	result := make([]*WalletStats, 0, len(w.cfg.TrackedWallets))
	for _, addr := range w.cfg.TrackedWallets {
		if stats, ok := w.walletStats[addr]; ok {
			result = append(result, stats)
		}
	}
	return result
}

// TrackedCount returns the number of currently tracked wallets
func (w *WalletTracker) TrackedCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.cfg.TrackedWallets)
}

// parseTransaction extracts swap details from a transaction
func (w *WalletTracker) parseTransaction(ctx context.Context, walletAddr, signature string) *types.WalletActivity {
	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return nil
	}

	tx, err := w.rpcClient.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{
		MaxSupportedTransactionVersion: uintPtr(0),
	})
	if err != nil || tx == nil {
		return nil
	}

	activity := w.extractSwapInfo(walletAddr, tx)
	if activity != nil {
		activity.TxSignature = signature
		activity.Timestamp = time.Now()
	}
	return activity
}

// extractSwapInfo parses transaction for swap details
func (w *WalletTracker) extractSwapInfo(walletAddr string, tx *rpc.GetTransactionResult) *types.WalletActivity {
	if tx.Meta == nil {
		return nil
	}

	dexPrograms := map[string]bool{
		"JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4": true,
		"JUP4Fb2cqiRUcaTHdrPC8h2gNsA2ETXiPDD33WcGuJB": true,
		"675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8": true,
		"whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc":  true,
	}

	decodedTx, err := tx.Transaction.GetTransaction()
	if err != nil {
		return nil
	}

	isDexTx := false
	for _, key := range decodedTx.Message.AccountKeys {
		if dexPrograms[key.String()] {
			isDexTx = true
			break
		}
	}
	if !isDexTx {
		return nil
	}

	preBalances := tx.Meta.PreTokenBalances
	postBalances := tx.Meta.PostTokenBalances

	var solChange float64
	var tokenAddress string
	var tokenChange float64

	walletIndex := -1
	for i, key := range decodedTx.Message.AccountKeys {
		if key.String() == walletAddr {
			walletIndex = i
			break
		}
	}

	if walletIndex >= 0 && len(tx.Meta.PreBalances) > walletIndex && len(tx.Meta.PostBalances) > walletIndex {
		preSol := float64(tx.Meta.PreBalances[walletIndex]) / 1e9
		postSol := float64(tx.Meta.PostBalances[walletIndex]) / 1e9
		solChange = postSol - preSol
	}

	preTokenMap := make(map[string]float64)
	for _, bal := range preBalances {
		if bal.Owner != nil && bal.Owner.String() == walletAddr {
			amount := 0.0
			if bal.UiTokenAmount != nil && bal.UiTokenAmount.UiAmount != nil {
				amount = *bal.UiTokenAmount.UiAmount
			}
			preTokenMap[bal.Mint.String()] = amount
		}
	}

	for _, bal := range postBalances {
		if bal.Owner != nil && bal.Owner.String() == walletAddr {
			mint := bal.Mint.String()
			postAmount := 0.0
			if bal.UiTokenAmount != nil && bal.UiTokenAmount.UiAmount != nil {
				postAmount = *bal.UiTokenAmount.UiAmount
			}
			change := postAmount - preTokenMap[mint]
			if change != 0 && mint != "So11111111111111111111111111111111111111112" {
				tokenAddress = mint
				tokenChange = change
			}
		}
	}

	var action string
	if tokenChange > 0 && solChange < 0 {
		action = "buy"
	} else if tokenChange < 0 && solChange > 0 {
		action = "sell"
	} else {
		return nil
	}

	return &types.WalletActivity{
		Wallet:       walletAddr,
		TokenAddress: tokenAddress,
		Action:       action,
		AmountSOL:    absFloat(solChange),
		TokenAmount:  absFloat(tokenChange),
	}
}

// CheckWalletHoldsToken checks if tracked wallets hold a specific token
func (w *WalletTracker) CheckWalletHoldsToken(ctx context.Context, tokenMint string) (int, []string) {
	holders := []string{}
	mint, err := solana.PublicKeyFromBase58(tokenMint)
	if err != nil {
		return 0, holders
	}

	w.mu.RLock()
	wallets := make([]string, len(w.cfg.TrackedWallets))
	copy(wallets, w.cfg.TrackedWallets)
	w.mu.RUnlock()

	for _, walletAddr := range wallets {
		wallet, err := solana.PublicKeyFromBase58(walletAddr)
		if err != nil {
			continue
		}
		ata, _, err := solana.FindAssociatedTokenAddress(wallet, mint)
		if err != nil {
			continue
		}
		balance, err := w.rpcClient.GetTokenAccountBalance(ctx, ata, rpc.CommitmentConfirmed)
		if err != nil || balance.Value == nil {
			continue
		}
		if balance.Value.UiAmount != nil && *balance.Value.UiAmount > 0 {
			holders = append(holders, walletAddr)
		}
	}
	return len(holders), holders
}

// FetchWalletPnL gets profit/loss from Birdeye
func (w *WalletTracker) FetchWalletPnL(ctx context.Context, walletAddr string) (float64, error) {
	url := fmt.Sprintf("https://public-api.birdeye.so/v1/wallet/token_profit?wallet=%s", walletAddr)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			TotalProfit float64 `json:"total_profit"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Data.TotalProfit, nil
}

// GetWalletStats returns stats for a tracked wallet
func (w *WalletTracker) GetWalletStats(walletAddr string) *WalletStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.walletStats[walletAddr]
}

func isValidSolanaAddress(addr string) bool {
	if len(addr) < 32 || len(addr) > 44 {
		return false
	}
	for _, c := range addr {
		if !((c >= '1' && c <= '9') || (c >= 'A' && c <= 'H') ||
			(c >= 'J' && c <= 'N') || (c >= 'P' && c <= 'Z') ||
			(c >= 'a' && c <= 'k') || (c >= 'm' && c <= 'z')) {
			return false
		}
	}
	return true
}

func intPtr(i int) *int       { return &i }
func uintPtr(u uint64) *uint64 { return &u }
func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
