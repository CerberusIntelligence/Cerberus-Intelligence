package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"solana-trading-bot/config"
	"solana-trading-bot/scanner"
	"solana-trading-bot/storage"
	"solana-trading-bot/types"
	"solana-trading-bot/wallettracker"

	log "github.com/sirupsen/logrus"
)

type Engine struct {
	cfg     *config.Config
	mu      sync.RWMutex
	scanner *scanner.Scanner
	store   *storage.Store

	positions        map[string]*types.Position
	history          []types.Trade
	balance          float64
	startBalance     float64    // balance at bot startup — used to compute session P&L
	sessionStartedAt time.Time  // used to filter session-only wins/losses
	lastTradeTimes []time.Time
	paused         bool

	wtracker *wallettracker.Tracker

	signalCh chan types.Signal
	alertCh  chan string
}

func New(cfg *config.Config) *Engine {
	e := &Engine{
		cfg:       cfg,
		scanner:   scanner.New(cfg),
		store:     storage.New("state.json"),
		positions:  make(map[string]*types.Position),
		history:    []types.Trade{},
		signalCh:   make(chan types.Signal, 100),
		alertCh:    make(chan string, 100),
	}
	e.wtracker = wallettracker.New(cfg, e.signalCh)

	if state, err := e.store.Load(); err == nil {
		e.positions = state.Positions
		e.history = state.History
		e.balance = state.Balance
		log.WithField("balance", fmt.Sprintf("%.4f SOL", e.balance)).Info("Loaded saved state")
	} else {
		e.balance = cfg.PaperBalance
		log.WithField("balance", fmt.Sprintf("%.4f SOL", e.balance)).Info("Starting fresh")
	}
	// Include open position capital in start baseline so session P&L reflects
	// only gains/losses made this session, not pre-existing position value.
	e.startBalance = e.balance
	for _, pos := range e.positions {
		e.startBalance += pos.EntryValueSOL
	}
	e.sessionStartedAt = time.Now()

	return e
}

func (e *Engine) SignalCh() chan<- types.Signal { return e.signalCh }
func (e *Engine) AlertCh() <-chan string        { return e.alertCh }

func (e *Engine) Start(ctx context.Context) {
	go e.monitorPositions(ctx)
	go e.periodicSave(ctx)
	go e.wtracker.Start(ctx)
	log.Info("Engine started — buy-everything mode")

	for {
		select {
		case <-ctx.Done():
			e.saveState()
			return
		case sig := <-e.signalCh:
			go e.handleSignal(ctx, sig)
		}
	}
}

// handleSignal: pure copy-trade — mirror wallet buys and sells instantly.
func (e *Engine) handleSignal(ctx context.Context, sig types.Signal) {
	if e.paused {
		return
	}

	// Wallet sold — close our position using tx-derived price
	if sig.IsSell {
		e.mu.Lock()
		pos, ok := e.positions[sig.Address]
		if !ok {
			e.mu.Unlock()
			return
		}
		posCopy := *pos
		delete(e.positions, sig.Address)
		e.mu.Unlock()

		price := sig.Price
		if price == 0 {
			// Fallback: fetch market price if tx price unavailable
			priceCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			price, _ = e.getPriceRace(priceCtx, sig.Address)
		}
		if price == 0 {
			price = posCopy.CurrentPrice
		}
		e.recordClose(posCopy, price, "wallet_sell")
		return
	}

	// Only gates: already holding this token, balance too low, or at position limit
	e.mu.Lock()
	if _, inPos := e.positions[sig.Address]; inPos {
		e.mu.Unlock()
		return
	}
	if len(e.positions) >= e.cfg.MaxPositions || e.balance < e.cfg.TradeAmountSOL/2 {
		e.mu.Unlock()
		return
	}
	e.mu.Unlock()

	e.enterPosition(ctx, sig, sig.Price, e.cfg.TradeAmountSOL)
}

// getPriceRace fetches price and liquidity from Jupiter and DexScreener simultaneously.
// Returns price and liquidity (liquidity only available from DexScreener).
func (e *Engine) getPriceRace(ctx context.Context, address string) (price float64, liquidityUSD float64) {
	type result struct {
		price     float64
		liquidity float64
	}
	ch := make(chan result, 2)

	priceCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	// Jupiter (price only)
	go func() {
		prices, err := e.scanner.GetPrices(priceCtx, []string{address})
		if err == nil && prices[address] > 0 {
			ch <- result{price: prices[address]}
		} else {
			ch <- result{}
		}
	}()

	// DexScreener (price + liquidity)
	go func() {
		info, err := e.scanner.FetchMarketData(priceCtx, address)
		if err == nil && info != nil && info.PriceSOL > 0 {
			ch <- result{price: info.PriceSOL, liquidity: info.LiquidityUSD}
		} else {
			ch <- result{}
		}
	}()

	// Collect both results — prefer DexScreener result for liquidity data
	var best result
	for i := 0; i < 2; i++ {
		r := <-ch
		if r.price > 0 && (best.price == 0 || r.liquidity > 0) {
			best = r
		}
	}
	return best.price, best.liquidity
}

func (e *Engine) enterPosition(ctx context.Context, sig types.Signal, price float64, amount float64) {
	qty := 0.0
	if price > 0 {
		qty = amount / price
	}

	pos := &types.Position{
		Address:       sig.Address,
		Symbol:        sig.Address[:8] + "...",
		EntryPrice:    price,
		CurrentPrice:  price,
		HighestPrice:  price,
		Quantity:      qty,
		EntryValueSOL: amount,
		OpenedAt:      time.Now(),
		Source:        sig.Source,
	}

	e.mu.Lock()
	if _, exists := e.positions[sig.Address]; exists {
		e.mu.Unlock()
		return
	}
	e.positions[sig.Address] = pos
	e.balance -= amount
	e.lastTradeTimes = append(e.lastTradeTimes, time.Now())
	e.mu.Unlock()

	mode := "PAPER"
	if !e.cfg.PaperTrading {
		mode = "LIVE"
	}

	log.WithFields(log.Fields{
		"mode":    mode,
		"address": sig.Address[:8] + "...",
		"price":   fmt.Sprintf("%.8f SOL", price),
		"amount":  fmt.Sprintf("%.4f SOL", amount),
		"source":  sig.Source,
	}).Info("BUY")

	e.sendAlert(fmt.Sprintf(
		"⚡ *BUY* [`%s`]\n"+
			"💰 Price: `%.8f SOL`\n"+
			"📊 Amount: `%.4f SOL`\n"+
			"📢 Source: `%s` | `%s`",
		sig.Address[:12]+"...",
		price, amount, sig.Source, mode,
	))

	// Enrich with token info in background — never blocks or exits the trade
	go e.enrichPosition(ctx, sig.Address)
}

// enrichPosition fetches token name/symbol/liquidity and updates the position for display.
// For wallet copy signals, entry price starts at 0 — this fills it in from market data.
// It NEVER exits the trade — stop-loss handles everything.
func (e *Engine) enrichPosition(ctx context.Context, address string) {
	enrichCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	info, err := e.scanner.FetchMarketData(enrichCtx, address)
	if err != nil || info == nil {
		log.WithField("address", address[:8]+"...").Warn("enrich FAILED — entry price unknown, sell will use last known price")
		return
	}

	e.mu.Lock()
	if pos, ok := e.positions[address]; ok {
		if info.Symbol != "" {
			pos.Symbol = info.Symbol
		}
		// Fill in entry price for copy-trade entries that bypassed the price fetch
		if pos.EntryPrice == 0 && info.PriceSOL > 0 {
			pos.EntryPrice = info.PriceSOL
			pos.CurrentPrice = info.PriceSOL
			pos.HighestPrice = info.PriceSOL
			pos.Quantity = pos.EntryValueSOL / info.PriceSOL
			log.WithFields(log.Fields{
				"address": address[:8] + "...",
				"symbol":  info.Symbol,
				"price":   fmt.Sprintf("%.8f SOL", info.PriceSOL),
			}).Info("enrich OK — entry price set")
		} else if pos.EntryPrice == 0 {
			log.WithField("address", address[:8]+"...").Warn("enrich OK but price=0 — P&L will be inaccurate")
		}
	}
	e.mu.Unlock()

	if info.Symbol != "" {
		e.sendAlert(fmt.Sprintf(
			"ℹ️ *Token Info* [%s]\n"+
				"💧 Liq: `$%.0f` | Vol 24h: `$%.0f`\n"+
				"📈 5m change: `%.1f%%`\n"+
				"🔒 Mint revoked: `%v`",
			info.Symbol,
			info.LiquidityUSD, info.Volume24h,
			info.PriceChange5m,
			info.MintRevoked,
		))
	}
}

// --- Position monitoring --------------------------------------------------

func (e *Engine) monitorPositions(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.checkPositions(ctx)
		}
	}
}

type posAction struct {
	pos     types.Position
	price   float64
	reason  string
	partial float64
}

func (e *Engine) checkPositions(ctx context.Context) {
	e.mu.RLock()
	if len(e.positions) == 0 {
		e.mu.RUnlock()
		return
	}
	addrs := make([]string, 0, len(e.positions))
	for addr := range e.positions {
		addrs = append(addrs, addr)
	}
	e.mu.RUnlock()

	prices, err := e.scanner.GetPrices(ctx, addrs)
	if err != nil {
		log.WithError(err).Warn("Price fetch failed")
		return
	}

	var timeouts []posAction

	e.mu.Lock()
	for addr, pos := range e.positions {
		price := prices[addr]
		if price > 0 {
			pos.CurrentPrice = price
			if price > pos.HighestPrice {
				pos.HighestPrice = price
			}
		}
		// Timeout: only exit if wallet never sells and token is dead
		if time.Since(pos.OpenedAt) > time.Duration(e.cfg.TimeoutMinutes)*time.Minute {
			timeouts = append(timeouts, posAction{pos: *pos, price: price, reason: "timeout"})
			delete(e.positions, addr)
		}
	}
	e.mu.Unlock()

	for _, a := range timeouts {
		e.recordClose(a.pos, a.price, a.reason)
	}
}

func (e *Engine) recordClose(pos types.Position, price float64, reason string) {
	currentValue := pos.Quantity * price
	if pos.Quantity == 0 || price == 0 {
		// No price data available — return entry capital, record as break-even
		currentValue = pos.EntryValueSOL
	}
	pnlSOL := currentValue - pos.EntryValueSOL
	pnlPct := 0.0
	if pos.EntryValueSOL > 0 {
		pnlPct = pnlSOL / pos.EntryValueSOL * 100
	}

	trade := types.Trade{
		Address:    pos.Address,
		Symbol:     pos.Symbol,
		Side:       "sell",
		EntryPrice: pos.EntryPrice,
		ExitPrice:  price,
		Quantity:   pos.Quantity,
		ValueSOL:   currentValue,
		PnLSOL:     pnlSOL,
		PnLPct:     pnlPct,
		Reason:     reason,
		Source:     pos.Source,
		OpenedAt:   pos.OpenedAt,
		ClosedAt:   time.Now(),
	}

	e.mu.Lock()
	e.history = append(e.history, trade)
	e.balance += currentValue
	e.mu.Unlock()

	emoji := "🔴"
	if pnlSOL > 0 {
		emoji = "🟢"
	}

	e.mu.RLock()
	sessionPnL := e.sessionPnL()
	wins, losses := 0, 0
	for _, t := range e.history {
		if t.ClosedAt.Before(e.sessionStartedAt) {
			continue
		}
		if t.PnLSOL > 0 {
			wins++
		} else if t.PnLSOL < 0 {
			losses++
		}
	}
	e.mu.RUnlock()

	log.WithFields(log.Fields{
		"symbol":      pos.Symbol,
		"pnl":         fmt.Sprintf("%.4f SOL (%.1f%%)", pnlSOL, pnlPct),
		"reason":      reason,
		"session_pnl": fmt.Sprintf("%+.4f SOL", sessionPnL),
		"w/l":         fmt.Sprintf("%d/%d", wins, losses),
	}).Info("SELL")

	e.sendAlert(fmt.Sprintf(
		"%s *SELL* [%s]\n"+
			"💰 Exit: `%.8f SOL`\n"+
			"📈 PnL: `%.4f SOL (%.1f%%)`\n"+
			"⏱ Held: `%s`\n"+
			"📋 Reason: `%s`",
		emoji, pos.Symbol,
		price, pnlSOL, pnlPct,
		time.Since(pos.OpenedAt).Round(time.Second),
		reason,
	))
}

func (e *Engine) recordPartialClose(pos types.Position, price float64, sellPct float64, reason string) {
	sellQty := pos.Quantity * sellPct
	sellValue := sellQty * price
	costBasis := pos.EntryValueSOL * sellPct
	pnlSOL := sellValue - costBasis
	pnlPct := 0.0
	if costBasis > 0 {
		pnlPct = pnlSOL / costBasis * 100
	}
	multiplier := price / pos.EntryPrice

	log.WithFields(log.Fields{
		"symbol":     pos.Symbol,
		"multiplier": fmt.Sprintf("%.1fx", multiplier),
		"pnl":        fmt.Sprintf("%.4f SOL", pnlSOL),
	}).Info("PARTIAL SELL")

	e.sendAlert(fmt.Sprintf(
		"🟡 *PARTIAL SELL* [%s] (%.0f%%)\n"+
			"💰 Price: `%.8f SOL` (%.1fx)\n"+
			"📈 PnL: `%.4f SOL (%.1f%%)`\n"+
			"📋 `%s`",
		pos.Symbol, sellPct*100,
		price, multiplier, pnlSOL, pnlPct, reason,
	))
}

// --- Helpers --------------------------------------------------------------

// sessionPnL returns the net profit/loss since bot startup.
// Total portfolio value = available balance + current market value of open positions.
func (e *Engine) sessionPnL() float64 {
	total := e.balance
	for _, pos := range e.positions {
		if pos.CurrentPrice > 0 {
			total += pos.Quantity * pos.CurrentPrice
		} else {
			total += pos.EntryValueSOL
		}
	}
	return total - e.startBalance
}

func (e *Engine) sendAlert(msg string) {
	select {
	case e.alertCh <- msg:
	default:
	}
}

func (e *Engine) checkRateLimit() bool {
	cutoff := time.Now().Add(-time.Hour)
	e.mu.Lock()
	defer e.mu.Unlock()
	recent := e.lastTradeTimes[:0]
	for _, t := range e.lastTradeTimes {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	e.lastTradeTimes = recent
	return len(recent) < e.cfg.MaxTradesPerHour
}

func (e *Engine) periodicSave(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.saveState()
		}
	}
}

func (e *Engine) saveState() {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sessionPnL := e.sessionPnL()
	emoji := "+"
	if sessionPnL < 0 {
		emoji = "-"
	}
	_ = emoji

	wins, losses := 0, 0
	for _, t := range e.history {
		if t.ClosedAt.Before(e.sessionStartedAt) {
			continue
		}
		if t.PnLSOL > 0 {
			wins++
		} else if t.PnLSOL < 0 {
			losses++
		}
	}

	log.WithFields(log.Fields{
		"balance":     fmt.Sprintf("%.4f SOL", e.balance),
		"open":        len(e.positions),
		"session_pnl": fmt.Sprintf("%+.4f SOL", sessionPnL),
		"wins":        wins,
		"losses":      losses,
	}).Info("Session snapshot")

	e.store.Save(&storage.State{
		Balance:   e.balance,
		Positions: e.positions,
		History:   e.history,
	})
}

// --- Public API -----------------------------------------------------------

func (e *Engine) GetStatus() (float64, map[string]*types.Position, []types.Trade) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	posCopy := make(map[string]*types.Position, len(e.positions))
	for k, v := range e.positions {
		cp := *v
		posCopy[k] = &cp
	}
	hist := make([]types.Trade, len(e.history))
	copy(hist, e.history)
	return e.balance, posCopy, hist
}

func (e *Engine) Pause()         { e.mu.Lock(); e.paused = true; e.mu.Unlock() }
func (e *Engine) Resume()        { e.mu.Lock(); e.paused = false; e.mu.Unlock() }
func (e *Engine) IsPaused() bool { e.mu.RLock(); defer e.mu.RUnlock(); return e.paused }

func (e *Engine) ClosePosition(address string) error {
	e.mu.Lock()
	pos, ok := e.positions[address]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("position not found")
	}
	posCopy := *pos
	delete(e.positions, address)
	e.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	price, err := e.scanner.GetPrice(ctx, address)
	if err != nil || price == 0 {
		price = posCopy.CurrentPrice
	}
	e.recordClose(posCopy, price, "manual")
	return nil
}

func (e *Engine) SetStopLoss(pct float64)         { e.cfg.StopLossPct = pct }
func (e *Engine) GetConfig() *config.Config        { return e.cfg }
func (e *Engine) GetSessionStartedAt() time.Time   { return e.sessionStartedAt }

func (e *Engine) AddTrackedWallet(address string) {
	e.wtracker.AddWallet(address)
}

func (e *Engine) GetTrackedWallets() []string {
	return e.wtracker.GetWallets()
}

func (e *Engine) InjectSignal(address, source string) {
	select {
	case e.signalCh <- types.Signal{
		Address:   address,
		Source:    source,
		Message:   "manual",
		Timestamp: time.Now(),
	}:
	default:
	}
}
