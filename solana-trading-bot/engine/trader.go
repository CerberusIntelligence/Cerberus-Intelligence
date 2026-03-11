package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"solana-trading-bot/config"
	"solana-trading-bot/safety"
	"solana-trading-bot/scanner"
	"solana-trading-bot/social"
	solanaPkg "solana-trading-bot/solana"
	"solana-trading-bot/strategy"
	"solana-trading-bot/telegram"
	"solana-trading-bot/tracker"
	"solana-trading-bot/trading"
	"solana-trading-bot/types"

	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// Trader is the main trading engine
type Trader struct {
	cfg           *config.Config
	log           *logrus.Logger

	// Components
	solana        *solanaPkg.Client
	executor      trading.Executor // Can be Jupiter (live) or PaperTrader
	safetyChecker *safety.Checker
	walletTracker *tracker.WalletTracker
	twitterMon    *social.TwitterMonitor
	tgMonitor     *telegram.Monitor
	tgBot         *telegram.Bot
	smartScanner  *scanner.SmartWalletScanner

	// Advanced Strategy Modules
	entryStrategy    *strategy.EntryStrategy
	exitStrategy     *strategy.ExitStrategy
	rugProtection    *strategy.RugProtection
	analytics        *strategy.Analytics
	positionManager  *strategy.PositionManager
	signalAggregator *strategy.SignalAggregator

	// Scheduled reporting
	reporter *Reporter

	// Adaptive learner — adjusts position sizes from win/loss history
	learner *strategy.Learner

	// State
	positions     map[string]*types.Position
	trades        []*types.Trade
	stats         *types.BotStats
	mu            sync.RWMutex

	// Flow-based distribution tracking (per token)
	distributionWindow time.Duration
	distribution       map[string]*distributionStats

	// LP history for drop detection
	lpHistory map[string]lpSample

	// Rate limiting
	limiter       *rate.Limiter
	tradeCount    int
	lastTradeTime time.Time

	// Cache for quick lookups
	tokenCache    *cache.Cache

	// Control
	paused        bool
	stopChan      chan struct{}
}

// NewTrader creates a new trading engine (for backwards compatibility)
func NewTrader(
	cfg *config.Config,
	log *logrus.Logger,
	solana *solanaPkg.Client,
	jupiter *solanaPkg.Jupiter,
	safetyChecker *safety.Checker,
	walletTracker *tracker.WalletTracker,
	twitterMon *social.TwitterMonitor,
	tgMonitor *telegram.Monitor,
	tgBot *telegram.Bot,
) *Trader {
	return NewTraderWithExecutor(cfg, log, solana, jupiter, safetyChecker, walletTracker, twitterMon, tgMonitor, tgBot, nil)
}

// NewTraderWithExecutor creates a new trading engine with custom executor
func NewTraderWithExecutor(
	cfg *config.Config,
	log *logrus.Logger,
	solana *solanaPkg.Client,
	executor trading.Executor,
	safetyChecker *safety.Checker,
	walletTracker *tracker.WalletTracker,
	twitterMon *social.TwitterMonitor,
	tgMonitor *telegram.Monitor,
	tgBot *telegram.Bot,
	smartScanner *scanner.SmartWalletScanner,
) *Trader {
	modeStr := "LIVE"
	if cfg.PaperTrading {
		modeStr = "PAPER"
	}
	log.WithField("mode", modeStr).Info("Creating trading engine")

	// Initialize advanced strategy modules
	entryStrat := strategy.NewEntryStrategy(cfg, log)
	exitStrat := strategy.NewExitStrategy(cfg, log)
	rugProt := strategy.NewRugProtection(cfg, log)
	analytics := strategy.NewAnalytics(cfg, log)
	posMgr := strategy.NewPositionManager(cfg, log)
	sigAgg := strategy.NewSignalAggregator(cfg, log)

	reporter := NewReporter(cfg, log, tgBot, analytics)
	learner := strategy.NewLearner(log, "learner_state.json")

	t := &Trader{
		cfg:              cfg,
		log:              log,
		solana:           solana,
		executor:         executor,
		safetyChecker:    safetyChecker,
		walletTracker:    walletTracker,
		twitterMon:       twitterMon,
		tgMonitor:        tgMonitor,
		tgBot:            tgBot,
		smartScanner:     smartScanner,
		entryStrategy:    entryStrat,
		exitStrategy:     exitStrat,
		rugProtection:    rugProt,
		analytics:        analytics,
		positionManager:  posMgr,
		signalAggregator: sigAgg,
		reporter:         reporter,
		learner:          learner,
		positions:        make(map[string]*types.Position),
		trades:           make([]*types.Trade, 0),
		stats: &types.BotStats{
			StartedAt: time.Now(),
		},
		limiter:           rate.NewLimiter(rate.Every(time.Duration(cfg.CooldownSeconds)*time.Second), 1),
		tokenCache:        cache.New(5*time.Minute, 10*time.Minute),
		stopChan:          make(chan struct{}),
		distributionWindow: time.Duration(cfg.LpDropWindowSeconds) * time.Second,
		distribution:       make(map[string]*distributionStats),
		lpHistory:          make(map[string]lpSample),
	}

	// Wire up wallet tracker notification callbacks
	if walletTracker != nil {
		walletTracker.OnWalletAdded = func(addr, reason string) {
			short := addr[:8] + "..." + addr[len(addr)-4:]
			msg := fmt.Sprintf("🐋 *COPY TRADING STARTED*\n\n👛 Wallet: `%s`\n📋 Reason: _%s_\n⏱ Polling every 15s for new trades", short, reason)
			if tgBot != nil {
				tgBot.SendReport(msg)
			}
		}
		walletTracker.OnWalletRemoved = func(addr, reason string) {
			short := addr[:8] + "..." + addr[len(addr)-4:]
			msg := fmt.Sprintf("🔄 *WALLET ROTATED OUT*\n\n👛 Wallet: `%s`\n📋 Reason: _%s_\n_Replacing with a better performer..._", short, reason)
			if tgBot != nil {
				tgBot.SendReport(msg)
			}
		}
	}

	// Wire up scanner callbacks
	if smartScanner != nil {
		// Auto-rotation: swap out underperformers after each scan
		smartScanner.OnScanComplete = func(topWallets []string) {
			if walletTracker == nil {
				return
			}
			added, removed := walletTracker.AutoRotate(topWallets)
			if len(added) > 0 || len(removed) > 0 {
				log.WithFields(logrus.Fields{
					"added":   len(added),
					"removed": len(removed),
				}).Info("Wallet auto-rotation complete")
			}
		}

		// Telegram blast: list all found wallets after every scan
		smartScanner.OnWalletsFound = func(wallets []*scanner.WalletPerformance) {
			if tgBot == nil {
				return
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("🔍 *WALLET SCAN COMPLETE* — %d found\n\n", len(wallets)))

			limit := len(wallets)
			if limit > 15 {
				limit = 15
			}
			for i, w := range wallets[:limit] {
				short := w.Address[:8] + "..." + w.Address[len(w.Address)-4:]
				grade := "🟡"
				if w.WinRate >= 0.70 {
					grade = "🟢"
				} else if w.WinRate < 0.55 {
					grade = "🔴"
				}
				sb.WriteString(fmt.Sprintf("%s #%d `%s`\n    WR: *%.0f%%* | PnL: *%.2f SOL* | Trades: %d\n\n",
					grade, i+1, short, w.WinRate*100, w.TotalPnL, w.TotalTrades))
			}
			if len(wallets) > 15 {
				sb.WriteString(fmt.Sprintf("_...and %d more_\n\n", len(wallets)-15))
			}
			sb.WriteString("_Use /copy\\_smart to follow the top wallets automatically_")
			tgBot.SendReport(sb.String())
		}
	}

	return t
}

// distributionStats tracks recent wallet flow for a token
type distributionStats struct {
	Events []walletFlowEvent
}

type walletFlowEvent struct {
	Time   time.Time
	Side   string  // "buy" or "sell"
	Amount float64 // in SOL
}

// lpSample stores last known LP liquidity for a token
type lpSample struct {
	LiquidityUSD float64
	Time         time.Time
}

// Start begins the trading engine
func (t *Trader) Start(ctx context.Context) error {
	modeStr := "LIVE"
	if t.cfg.PaperTrading {
		modeStr = "PAPER"
	}
	t.log.WithField("mode", modeStr).Info("Starting trading engine")

	// Start processing signals
	go t.processSignals(ctx)

	// Start wallet activity processing
	go t.processWalletActivity(ctx)

	// Start position monitor
	go t.monitorPositions(ctx)

	// Start command processor
	go t.processCommands(ctx)

	// Start rug protection alert processor
	go t.processRugAlerts(ctx)

	// Start signal cleanup routine
	go t.cleanupRoutine(ctx)

	// Start scheduled reporter (6am / 12pm / 10pm)
	go t.reporter.Start(ctx)

	// Send startup message with full command list
	if t.tgBot != nil {
		go t.tgBot.SendReport(t.buildCommandList())
	}

	return nil
}

// buildCommandList returns a Telegram-formatted message of all available commands
func (t *Trader) buildCommandList() string {
	mode := "PAPER"
	if !t.cfg.PaperTrading {
		mode = "🔴 LIVE"
	}
	return fmt.Sprintf(`🤖 *BOT ONLINE* — %s MODE

*BOT CONTROL*
/start — Resume trading
/stop — Pause trading

*INFO & STATS*
/status — Win rate, P&L summary
/positions — Open positions
/balance — SOL wallet balance
/analytics — Detailed breakdown
/portfolio — Full portfolio view
/heat — Portfolio risk heat map

*COPY TRADING*
/wallets — Tracked wallets \+ stats
/smart\_wallets — Discovered candidates
/copy\_smart — Auto\-follow top wallets
/add\_wallet \<addr\> — Manually add wallet
/remove\_wallet \<addr\> — Remove wallet

*REPORTS*
/report — Trigger report now
/learn — Learner status \+ multipliers

*PAPER TRADING*
/paper\_stats — Paper performance
/paper\_reset — Reset paper balance

*MANUAL TRADING*
/check \<addr\> — Safety check a token
/sell \<addr\> — Sell a position
/sell\_all — Emergency exit all`, mode)
}

// Stop gracefully stops the trader
func (t *Trader) Stop() {
	close(t.stopChan)
}

// Pause temporarily pauses trading
func (t *Trader) Pause() {
	t.mu.Lock()
	t.paused = true
	t.mu.Unlock()
	t.log.Info("Trading paused")
}

// Resume resumes trading
func (t *Trader) Resume() {
	t.mu.Lock()
	t.paused = false
	t.mu.Unlock()
	t.log.Info("Trading resumed")
}

// processSignals handles incoming Telegram signals
func (t *Trader) processSignals(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopChan:
			return
		case signal := <-t.tgMonitor.Signals():
			go t.handleSignal(ctx, signal)
		}
	}
}

// handleSignal processes a single signal
func (t *Trader) handleSignal(ctx context.Context, signal *types.Signal) {
	t.log.WithFields(logrus.Fields{
		"token":   signal.TokenAddress,
		"channel": signal.ChannelName,
	}).Info("Processing signal")

	// Check if paused
	if t.isPaused() {
		t.log.Debug("Trading paused, skipping signal")
		return
	}

	// Check rate limits
	if !t.canTrade() {
		t.log.Debug("Rate limited, skipping signal")
		return
	}

	// Reject stale signals — buying a token 10 minutes after the call means
	// the pump already happened and we're buying the top.
	if t.cfg.SignalMaxAgeMinutes > 0 {
		signalAge := time.Since(signal.ReceivedAt)
		if signalAge > time.Duration(t.cfg.SignalMaxAgeMinutes)*time.Minute {
			t.log.WithFields(logrus.Fields{
				"age_seconds": int(signalAge.Seconds()),
				"max_minutes": t.cfg.SignalMaxAgeMinutes,
				"channel":     signal.ChannelName,
			}).Info("Signal too old — skipping to avoid buying pumped token")
			return
		}
	}

	// Check if already have position
	if t.hasPosition(signal.TokenAddress) {
		t.log.Debug("Already have position, skipping")
		return
	}

	// Get token info
	var token *types.Token
	var err error

	if t.solana != nil {
		token, err = t.solana.GetTokenInfo(ctx, signal.TokenAddress)
		if err != nil {
			t.log.WithError(err).Error("Failed to get token info")
			// For paper trading, create minimal token info
			if t.cfg.PaperTrading {
				token = &types.Token{
					Address:      signal.TokenAddress,
					Symbol:       signal.TokenAddress[:6],
					Name:         "Unknown Token",
					Decimals:     9,
					DiscoveredAt: time.Now(),
				}
			} else {
				return
			}
		}
	} else if t.cfg.PaperTrading {
		// Paper mode without Solana client
		token = &types.Token{
			Address:      signal.TokenAddress,
			Symbol:       signal.TokenAddress[:6],
			Name:         "Unknown Token",
			Decimals:     9,
			DiscoveredAt: time.Now(),
		}
	}

	token.Source = signal.ChannelName

	// Pre-populate executor's token cache so Buy() doesn't make a redundant RPC call
	if jup, ok := t.executor.(*solanaPkg.Jupiter); ok {
		jup.CacheToken(token)
	}

	// ========== ADVANCED ENTRY ANALYSIS ==========

	// 1. Check entry timing with entry strategy
	entrySignal := t.entryStrategy.AnalyzeEntry(ctx, signal.TokenAddress)
	if !entrySignal.ShouldEnter {
		t.log.WithField("reason", entrySignal.Reason).Info("Entry strategy rejected")
		return
	}

	// 2. Check if should wait for dip
	if entrySignal.WaitForDip {
		t.log.WithField("target", entrySignal.TargetDipPrice).Info("Waiting for dip entry")
		// Register for dip monitoring (would need additional implementation)
	}

	// 3. Add to signal aggregator
	t.signalAggregator.AddSignal(signal.TokenAddress, signal.ChannelName, "telegram", signal.Confidence, nil)

	// 4. Get aggregated signal strength
	signalStrength := t.signalAggregator.GetSignalStrength(ctx, signal.TokenAddress)
	if signalStrength.Score < 50 && !signalStrength.ShouldTrade {
		t.log.WithFields(logrus.Fields{
			"score":   signalStrength.Score,
			"reasons": signalStrength.Reasons,
		}).Info("Aggregated signal too weak")
		// Continue anyway for paper trading to test
	}

	// 5. Record signal for analytics
	currentPrice := 0.0
	if t.solana != nil {
		currentPrice, _ = t.solana.GetTokenPrice(ctx, signal.TokenAddress)
	}
	t.analytics.RecordSignal(signal.TokenAddress, signal.ChannelName, currentPrice)

	// 6. Check channel performance history
	shouldSkip, skipReason := t.analytics.ShouldSkipBasedOnHistory(signal.ChannelName)
	if shouldSkip {
		t.log.WithField("reason", skipReason).Info("Skipping based on channel history")
		return
	}

	// 7. Check rug protection - detect bundles/snipers
	bundleInfo := t.rugProtection.DetectBundle(ctx, signal.TokenAddress)
	if bundleInfo != nil && bundleInfo.LaunchSuspicious {
		t.log.WithFields(logrus.Fields{
			"bundled": bundleInfo.BundledPercent,
			"snipers": bundleInfo.SniperDetected,
		}).Warn("Suspicious launch detected - reducing position")
		entrySignal.RecommendedSize *= 0.5
	}

	// 8. Check trading time
	isGoodTime, timeConf := t.analytics.IsGoodTradingTime()
	if !isGoodTime && timeConf < 0.4 {
		t.log.Info("Not ideal trading time - reducing position")
		entrySignal.RecommendedSize *= 0.75
	}

	// ========== SAFETY CHECKS ==========

	// Run safety checks
	safetyResult := t.safetyChecker.CheckToken(ctx, token)

	// LP drop guard: abort if LP has dropped too much in the recent window
	if t.isLpDropTooHigh(token.Address, safetyResult.LiquidityUSD) {
		t.log.WithFields(logrus.Fields{
			"token":      token.Symbol,
			"liquidity":  safetyResult.LiquidityUSD,
			"threshold":  t.cfg.MaxLpDropPercent,
			"window_sec": t.cfg.LpDropWindowSeconds,
		}).Warn("Aborting entry: LP dropped too much in recent window")
		return
	}

	// Add analytics bonus to safety score
	analyticsBonus := t.analytics.GetSignalBonus(signal.TokenAddress, signal.ChannelName)
	safetyResult.Score += analyticsBonus

	// Notify about signal
	if err := t.tgBot.SendSignalAlert(signal, safetyResult); err != nil {
		t.log.WithError(err).Warn("Failed to send signal alert")
	}

	// Check if passes safety
	if !safetyResult.IsValid {
		t.log.WithField("reasons", safetyResult.Reasons).Info("Token failed safety checks")
		return
	}

	// Additional validation: check if tracked wallets hold this token
	if t.cfg.WalletTrackingEnabled {
		holdersCount, _ := t.walletTracker.CheckWalletHoldsToken(ctx, signal.TokenAddress)
		safetyResult.TrackedWalletsBought = holdersCount
		if holdersCount > 0 {
			safetyResult.SmartMoneyIn = true
			t.log.WithField("wallets", holdersCount).Info("Tracked wallets hold this token")
		}
	}

	// ========== POSITION SIZING ==========

	// Calculate position size with position manager
	positionResult := t.positionManager.CalculatePositionSize(ctx, signal.TokenAddress, entrySignal.Confidence, t.cfg.PortfolioSize)
	if positionResult.RecommendedSize <= 0 {
		t.log.WithField("reason", positionResult.Reason).Info("Position manager rejected trade")
		return
	}

	// Adjust based on entry signal recommendation
	finalSizeMultiplier := positionResult.RecommendedSize * entrySignal.RecommendedSize

	// Execute buy with adjusted size
	t.executeBuyWithSize(ctx, token, safetyResult, "telegram_signal", finalSizeMultiplier)

	// Start rug protection monitoring for this token
	t.rugProtection.StartMonitoring(ctx, signal.TokenAddress)
}

// processWalletActivity handles tracked wallet activities
func (t *Trader) processWalletActivity(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopChan:
			return
		case activity := <-t.walletTracker.Activities():
			go t.handleWalletActivity(ctx, activity)
		}
	}
}

// handleWalletActivity processes wallet activity
func (t *Trader) handleWalletActivity(ctx context.Context, activity *types.WalletActivity) {
	t.log.WithFields(logrus.Fields{
		"wallet": activity.Wallet[:8] + "...",
		"action": activity.Action,
		"token":  activity.TokenAddress,
	}).Info("Wallet activity detected")

	// Get token info for notification — fetch once and reuse for copy trade path
	symbol := "UNKNOWN"
	var cachedToken *types.Token
	if t.solana != nil {
		if tok, err := t.solana.GetTokenInfo(ctx, activity.TokenAddress); err == nil {
			symbol = tok.Symbol
			cachedToken = tok
			// Pre-populate executor cache so Sell/Buy won't re-fetch
			if jup, ok := t.executor.(*solanaPkg.Jupiter); ok {
				jup.CacheToken(tok)
			}
		}
	}

	// Notify
	t.tgBot.SendWalletAlert(activity, symbol)

	// Update distribution stats for this token
	t.updateDistribution(activity)

	if activity.Action == "buy" && !t.hasPosition(activity.TokenAddress) {
		// Consider copy trading
		if t.isPaused() || !t.canTrade() {
			return
		}

		// Quick safety check
		ok, reason := t.safetyChecker.QuickCheck(ctx, activity.TokenAddress)
		if !ok {
			t.log.WithField("reason", reason).Info("Wallet buy failed quick check")
			return
		}

		// Reuse the token info fetched above — avoids a second GetTokenInfo RPC call
		var token *types.Token
		if cachedToken != nil {
			token = cachedToken
		} else if t.cfg.PaperTrading {
			token = &types.Token{
				Address:      activity.TokenAddress,
				Symbol:       activity.TokenAddress[:6],
				Name:         "Unknown Token",
				Decimals:     9,
				DiscoveredAt: time.Now(),
			}
		} else {
			// Live mode and no token info — fetch as last resort
			var err error
			token, err = t.solana.GetTokenInfo(ctx, activity.TokenAddress)
			if err != nil {
				return
			}
		}

		if token == nil && t.cfg.PaperTrading {
			token = &types.Token{
				Address:      activity.TokenAddress,
				Symbol:       activity.TokenAddress[:6],
				Name:         "Unknown Token",
				Decimals:     9,
				DiscoveredAt: time.Now(),
			}
		}

		token.Source = "wallet_copy:" + activity.Wallet[:8]

		safetyResult := t.safetyChecker.CheckToken(ctx, token)
		if safetyResult.IsValid {
			t.executeBuy(ctx, token, safetyResult, "wallet_copy")
		}
	}
}

// executeBuy performs the actual buy (backwards compatible)
func (t *Trader) executeBuy(ctx context.Context, token *types.Token, safety *types.TokenSafety, reason string) {
	t.executeBuyWithSize(ctx, token, safety, reason, 1.0)
}

// executeBuyWithSize performs a buy with custom size multiplier
func (t *Trader) executeBuyWithSize(ctx context.Context, token *types.Token, safety *types.TokenSafety, reason string, sizeMultiplier float64) {
	// Calculate dynamic position size with LP-aware caps
	positionSize := t.computePositionSize(ctx, safety, sizeMultiplier)
	if positionSize <= 0 {
		t.log.WithField("token", token.Symbol).Info("Position size computed as zero, skipping buy")
		return
	}

	// Apply learned multiplier — grows positions on high-performing sources,
	// shrinks on poor performers. Defaults to 1.0 until enough data.
	learnedMult := t.learner.GetPositionMultiplier(token.Source)
	if learnedMult != 1.0 {
		t.log.WithFields(logrus.Fields{
			"token":  token.Symbol,
			"source": token.Source,
			"mult":   fmt.Sprintf("%.2fx", learnedMult),
		}).Info("Applying learned position multiplier")
		positionSize *= learnedMult
		if t.cfg.MaxPositionSize > 0 && positionSize > t.cfg.MaxPositionSize {
			positionSize = t.cfg.MaxPositionSize
		}
	}

	modePrefix := ""
	if t.cfg.PaperTrading {
		modePrefix = "[PAPER] "
	}

	t.log.WithFields(logrus.Fields{
		"token":      token.Symbol,
		"amount":     positionSize,
		"reason":     reason,
		"multiplier": sizeMultiplier,
		"mode":       modePrefix,
	}).Info(modePrefix + "Executing buy")

	// Execute via executor (Jupiter or PaperTrader)
	trade, err := t.executor.Buy(ctx, token.Address, positionSize)
	if err != nil {
		t.log.WithError(err).Error(modePrefix + "Buy failed")
		t.tgBot.SendError(err, modePrefix+"Buy execution failed for "+token.Symbol)
		return
	}

	trade.Token = token
	trade.Reason = reason

	// Create position — store entry context for learning feedback at close time
	position := &types.Position{
		ID:              generatePositionID(),
		Token:           token,
		EntryPrice:      trade.Price,
		CurrentPrice:    trade.Price,
		Quantity:        trade.Quantity,
		EntryValueSOL:   trade.ValueSOL,
		CurrentValueSOL: trade.ValueSOL,
		HighestPrice:    trade.Price,
		InvestedSOL:     positionSize,
		OpenedAt:        time.Now(),
		Status:          types.PositionOpen,
	}
	if safety != nil {
		position.EntryLiquidityUSD = safety.LiquidityUSD
		position.EntryHolderCount = safety.TotalHolders
		position.EntrySafetyScore = safety.Score
	}

	// Store position and trade
	t.mu.Lock()
	t.positions[token.Address] = position
	t.trades = append(t.trades, trade)
	t.stats.TotalTrades++
	t.lastTradeTime = time.Now()
	t.tradeCount++
	t.mu.Unlock()

	// Register with advanced modules
	t.exitStrategy.RegisterPosition(token.Address, trade.Price)
	t.positionManager.RegisterPosition(token.Address, token.Symbol, trade.Price, positionSize)

	// Record entry for scheduled report
	t.reporter.RecordBuy(trade)

	// Notify
	t.tgBot.SendTradeAlert(trade)

	t.log.WithFields(logrus.Fields{
		"token": token.Symbol,
		"tx":    trade.TxSignature,
		"qty":   trade.Quantity,
	}).Info(modePrefix + "Buy executed successfully")
}

// monitorPositions watches open positions for exit conditions
func (t *Trader) monitorPositions(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopChan:
			return
		case <-ticker.C:
			t.checkPositions(ctx)
		}
	}
}

// checkPositions evaluates all positions for exit conditions
func (t *Trader) checkPositions(ctx context.Context) {
	t.mu.RLock()
	positions := make([]*types.Position, 0, len(t.positions))
	for _, p := range t.positions {
		if p.Status != types.PositionClosed {
			positions = append(positions, p)
		}
	}
	t.mu.RUnlock()

	for _, pos := range positions {
		t.evaluatePosition(ctx, pos)
	}
}

// evaluatePosition checks exit conditions for a position
func (t *Trader) evaluatePosition(ctx context.Context, pos *types.Position) {
	var price float64
	var err error

	// Get current price
	if t.solana != nil {
		price, err = t.solana.GetTokenPrice(ctx, pos.Token.Address)
		if err != nil {
			// For paper trading, simulate price movement
			if t.cfg.PaperTrading {
				// Use entry price with some variance
				price = pos.CurrentPrice
			} else {
				return
			}
		}
	} else if t.cfg.PaperTrading {
		price = pos.CurrentPrice
	}

	// Update position
	t.mu.Lock()
	pos.CurrentPrice = price
	pos.CurrentValueSOL = pos.Quantity * price / pos.EntryPrice * pos.EntryValueSOL
	pos.PnLPercent = (pos.CurrentPrice - pos.EntryPrice) / pos.EntryPrice
	pos.PnLSOL = pos.CurrentValueSOL - pos.EntryValueSOL

	if price > pos.HighestPrice {
		pos.HighestPrice = price
		// Update trailing stop
		pos.TrailingStopPrice = price * (1 - t.cfg.TrailingStopPercent)
	}
	t.mu.Unlock()

	// Check exit conditions
	t.checkExitConditions(ctx, pos)
}

// checkExitConditions determines if position should be closed
func (t *Trader) checkExitConditions(ctx context.Context, pos *types.Position) {
	// First, check high-speed distribution-based exit
	if t.cfg.DistributionThreshold > 0 {
		score := t.getDistributionScore(pos.Token.Address)
		if score >= t.cfg.DistributionThreshold {
			t.log.WithFields(logrus.Fields{
				"token": pos.Token.Symbol,
				"score": score,
			}).Warn("Distribution exit triggered - heavy net selling detected")
			t.executeClose(ctx, pos, 1.0, "distribution_exit")
			return
		}
	}

	// ========== ADVANCED EXIT STRATEGY ==========

	// Use advanced exit strategy module for intelligent exit decisions
	exitSignal := t.exitStrategy.AnalyzeExit(ctx, pos)

	if exitSignal != nil && exitSignal.ShouldExit {
		t.log.WithFields(logrus.Fields{
			"token":   pos.Token.Symbol,
			"reason":  exitSignal.Reason,
			"urgency": exitSignal.Urgency,
			"percent": exitSignal.ExitPercent,
		}).Info("Exit strategy triggered")

		t.executeClose(ctx, pos, exitSignal.ExitPercent, exitSignal.Reason)

		// Record partial sell in exit strategy
		if exitSignal.ExitPercent < 1.0 {
			t.exitStrategy.RecordPartialSell(pos.Token.Address, exitSignal.ExitPercent, exitSignal.Price, exitSignal.Reason)
		}
		return
	}

	// ========== FALLBACK CHECKS (original logic) ==========

	// 1. Check stop loss
	if pos.PnLPercent <= -t.cfg.StopLossPercent {
		t.log.WithField("pnl", pos.PnLPercent).Info("Stop loss triggered")
		t.executeClose(ctx, pos, 1.0, "stop_loss")
		return
	}

	// 2. Check trailing stop
	if pos.CurrentPrice <= pos.TrailingStopPrice && pos.TrailingStopPrice > 0 {
		t.log.Info("Trailing stop triggered")
		t.executeClose(ctx, pos, 1.0, "trailing_stop")
		return
	}

	// 3. Check take profit levels
	for i, tpLevel := range t.cfg.TakeProfitLevels {
		if pos.TPLevelHit > i {
			continue // Already hit this level
		}

		// TP level is multiplier (2.0 = 100% gain)
		targetGain := tpLevel - 1.0

		if pos.PnLPercent >= targetGain {
			sellPercent := t.cfg.TakeProfitPercents[i]
			t.log.WithFields(logrus.Fields{
				"level":   i + 1,
				"gain":    pos.PnLPercent,
				"selling": sellPercent,
			}).Info("Take profit triggered")

			t.executeClose(ctx, pos, sellPercent, fmt.Sprintf("take_profit_%dx", int(tpLevel)))

			t.mu.Lock()
			pos.TPLevelHit = i + 1
			t.mu.Unlock()
			break
		}
	}

	// 4. Check timeout
	if time.Since(pos.OpenedAt) > time.Duration(t.cfg.TimeoutMinutes)*time.Minute {
		// Only timeout if not in profit
		if pos.PnLPercent < 0.1 {
			t.log.Info("Position timeout triggered")
			t.executeClose(ctx, pos, 1.0, "timeout")
			return
		}
	}

	// 5. Check social sentiment (if enabled)
	if t.cfg.TwitterEnabled {
		shouldHold, reason := t.twitterMon.ShouldHold(pos.Token.Address)
		if !shouldHold && pos.PnLPercent > 0 {
			t.log.WithField("reason", reason).Info("Social signal suggests sell")
			t.executeClose(ctx, pos, 0.5, "social_signal_negative")
		}
	}

	// 6. Check for scale-in opportunity
	shouldScale, scaleAmount := t.positionManager.ShouldScaleIn(pos.Token.Address, pos.CurrentPrice)
	if shouldScale && t.canTrade() {
		t.log.WithFields(logrus.Fields{
			"token":  pos.Token.Symbol,
			"amount": scaleAmount,
		}).Info("Scaling into position on dip")

		// Execute scale-in buy
		if trade, err := t.executor.Buy(ctx, pos.Token.Address, scaleAmount); err == nil {
			t.positionManager.ScaleIn(pos.Token.Address, trade.Price, scaleAmount, "dip_scale")
			t.tgBot.SendTradeAlert(trade)
		}
	}
}

// executeClose sells a position
func (t *Trader) executeClose(ctx context.Context, pos *types.Position, percent float64, reason string) {
	sellAmount := pos.Quantity * percent * (1 - pos.AmountSold/pos.Quantity)

	if sellAmount <= 0 {
		return
	}

	modePrefix := ""
	if t.cfg.PaperTrading {
		modePrefix = "[PAPER] "
	}

	t.log.WithFields(logrus.Fields{
		"token":  pos.Token.Symbol,
		"amount": sellAmount,
		"reason": reason,
	}).Info(modePrefix + "Executing sell")

	trade, err := t.executor.Sell(ctx, pos.Token.Address, sellAmount, pos.Token.Decimals)
	if err != nil {
		t.log.WithError(err).Error(modePrefix + "Sell failed")
		t.tgBot.SendError(err, modePrefix+"Sell execution failed for "+pos.Token.Symbol)
		return
	}

	trade.Token = pos.Token
	trade.Reason = reason

	isFullClose := false

	// Update position
	t.mu.Lock()
	pos.AmountSold += sellAmount
	pos.RealizedPnL += trade.ValueSOL - (pos.EntryValueSOL * percent)

	if pos.AmountSold >= pos.Quantity*0.99 {
		pos.Status = types.PositionClosed
		delete(t.positions, pos.Token.Address)
		isFullClose = true

		// Update stats
		if pos.RealizedPnL > 0 {
			t.stats.WinningTrades++
			if pos.PnLPercent > t.stats.BestTrade {
				t.stats.BestTrade = pos.PnLPercent
			}
		} else {
			t.stats.LosingTrades++
			if pos.PnLPercent < t.stats.WorstTrade {
				t.stats.WorstTrade = pos.PnLPercent
			}
		}
		t.stats.TotalPnLSOL += pos.RealizedPnL
	} else {
		pos.Status = types.PositionPartial
	}

	t.trades = append(t.trades, trade)
	t.mu.Unlock()

	// Record trade for analytics
	t.analytics.RecordTrade(strategy.TradeRecord{
		TokenAddress: pos.Token.Address,
		Channel:      pos.Token.Source,
		EntryTime:    pos.OpenedAt,
		ExitTime:     time.Now(),
		EntryPrice:   pos.EntryPrice,
		ExitPrice:    trade.Price,
		PnLPercent:   pos.PnLPercent * 100,
		Success:      pos.RealizedPnL > 0,
	})

	// Record sell for scheduled report
	t.reporter.RecordSell(trade, pos.PnLPercent*100, pos.RealizedPnL, pos.RealizedPnL > 0)

	// Feed outcome to learner — updates position size multipliers for this source
	peakGainPct := 0.0
	if pos.EntryPrice > 0 {
		peakGainPct = (pos.HighestPrice - pos.EntryPrice) / pos.EntryPrice * 100
	}
	learnerNotif := t.learner.RecordOutcome(strategy.LearnerRecord{
		Channel:           pos.Token.Source,
		EntryLiquidityUSD: pos.EntryLiquidityUSD,
		EntryHolderCount:  pos.EntryHolderCount,
		EntrySafetyScore:  pos.EntrySafetyScore,
		EntryHour:         pos.OpenedAt.Hour(),
		PnLPercent:        pos.PnLPercent * 100,
		PeakGainPercent:   peakGainPct,
		ExitReason:        reason,
		HoldDurationMin:   time.Since(pos.OpenedAt).Minutes(),
		IsWin:             pos.RealizedPnL > 0,
	})
	if learnerNotif != "" && t.tgBot != nil {
		if err := t.tgBot.SendReport(learnerNotif); err != nil {
			t.log.WithError(err).Warn("Failed to send learner notification")
		}
	}

	// Update wallet tracker stats if this was a copy trade
	if strings.HasPrefix(pos.Token.Source, "wallet_copy:") && t.walletTracker != nil {
		walletAddr := strings.TrimPrefix(pos.Token.Source, "wallet_copy:")
		// Resolve short key back to full address
		for _, tracked := range t.walletTracker.GetTrackedWallets() {
			if strings.HasPrefix(tracked.Address, walletAddr) {
				t.walletTracker.RecordCopyTradeResult(tracked.Address, pos.RealizedPnL > 0, pos.RealizedPnL)
				break
			}
		}
	}

	// Clean up modules if fully closed
	if isFullClose {
		t.exitStrategy.ClosePosition(pos.Token.Address)
		t.positionManager.RemovePosition(pos.Token.Address)
		t.rugProtection.StopMonitoring(pos.Token.Address)
	}

	// Notify
	t.tgBot.SendTradeAlert(trade)
}

// processRugAlerts handles rug protection alerts
func (t *Trader) processRugAlerts(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopChan:
			return
		case alert := <-t.rugProtection.Alerts():
			t.handleRugAlert(ctx, alert)
		}
	}
}

// handleRugAlert processes a rug protection alert
func (t *Trader) handleRugAlert(ctx context.Context, alert *strategy.RugAlert) {
	t.log.WithFields(logrus.Fields{
		"token":    alert.TokenAddress[:8] + "...",
		"type":     alert.AlertType,
		"severity": alert.Severity,
	}).Warn(alert.Message)

	// Notify via Telegram
	t.tgBot.SendRugAlert(alert.TokenAddress, alert.Message, alert.Severity)

	// If should exit, execute sell
	if alert.ShouldExit {
		t.mu.RLock()
		pos, exists := t.positions[alert.TokenAddress]
		t.mu.RUnlock()

		if exists {
			t.log.WithFields(logrus.Fields{
				"token":   pos.Token.Symbol,
				"percent": alert.ExitPercent,
			}).Warn("Rug protection triggered exit")

			t.executeClose(ctx, pos, alert.ExitPercent, "rug_protection_"+alert.AlertType)
		}
	}
}

// cleanupRoutine periodically cleans up old data
func (t *Trader) cleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopChan:
			return
		case <-ticker.C:
			t.analytics.CleanupOldPatterns()
			t.signalAggregator.CleanupOldSignals()
		}
	}
}

// processCommands handles Telegram bot commands
func (t *Trader) processCommands(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopChan:
			return
		case cmd := <-t.tgBot.Commands():
			t.handleCommand(ctx, cmd)
		}
	}
}

// handleCommand processes bot commands
func (t *Trader) handleCommand(ctx context.Context, cmd telegram.Command) {
	switch cmd.Name {
	case "start":
		t.Resume()
	case "stop", "pause":
		t.Pause()
	case "status":
		t.tgBot.SendStats(t.stats)
	case "positions":
		t.sendPositionsSummary()
	case "balance":
		if t.solana != nil {
			balance, _ := t.solana.GetSOLBalance(ctx)
			t.log.WithField("balance", balance).Info("Balance checked")
		} else {
			t.log.Info("Paper trading mode - no real balance")
		}
	case "add_wallet":
		if len(cmd.Args) > 0 {
			t.walletTracker.AddWallet(cmd.Args[0])
		}
	case "remove_wallet":
		if len(cmd.Args) > 0 {
			t.walletTracker.RemoveWallet(cmd.Args[0])
		}
	case "sell":
		if len(cmd.Args) > 0 {
			t.manualSell(ctx, cmd.Args[0])
		}
	case "sell_all":
		t.sellAllPositions(ctx)
	case "paper_stats":
		t.sendPaperStats()
	case "paper_reset":
		t.resetPaperTrading()
	case "check":
		if len(cmd.Args) > 0 {
			t.checkToken(ctx, cmd.Args[0], cmd.Args)
		}
	case "smart_wallets":
		t.showSmartWallets()
	case "copy_smart":
		t.copySmartWallets()
	case "wallets":
		t.showTrackedWallets()
	case "analytics":
		t.showAnalytics()
	case "portfolio":
		t.showPortfolio()
	case "heat":
		t.showPortfolioHeat()
	case "report":
		go t.reporter.SendNow()
	case "learn":
		t.tgBot.SendReport(t.learner.StatusMessage())
	case "commands", "help":
		t.tgBot.SendReport(t.buildCommandList())
	}
}

// calculatePositionSize returns the SOL amount for a trade
func (t *Trader) calculatePositionSize() float64 {
	// 2% risk of portfolio
	size := t.cfg.PortfolioSize * t.cfg.RiskPerTrade

	// Cap at max position size
	if size > t.cfg.MaxPositionSize {
		size = t.cfg.MaxPositionSize
	}

	return size
}

// Helper methods
func (t *Trader) isPaused() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.paused
}

func (t *Trader) canTrade() bool {
	// Check rate limiter
	if !t.limiter.Allow() {
		return false
	}

	// Check hourly trade limit
	t.mu.RLock()
	defer t.mu.RUnlock()

	if time.Since(t.lastTradeTime) > time.Hour {
		t.tradeCount = 0
	}

	return t.tradeCount < t.cfg.MaxTradesPerHour
}

func (t *Trader) hasPosition(tokenAddress string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, exists := t.positions[tokenAddress]
	return exists
}

func (t *Trader) sendPositionsSummary() {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, pos := range t.positions {
		t.tgBot.SendPositionUpdate(pos)
	}
}

func (t *Trader) manualSell(ctx context.Context, tokenAddress string) {
	t.mu.RLock()
	pos, exists := t.positions[tokenAddress]
	t.mu.RUnlock()

	if !exists {
		return
	}

	t.executeClose(ctx, pos, 1.0, "manual_sell")
}

func (t *Trader) sellAllPositions(ctx context.Context) {
	t.mu.RLock()
	positions := make([]*types.Position, 0, len(t.positions))
	for _, p := range t.positions {
		positions = append(positions, p)
	}
	t.mu.RUnlock()

	for _, pos := range positions {
		t.executeClose(ctx, pos, 1.0, "manual_sell_all")
	}
}

func (t *Trader) sendPaperStats() {
	if !t.cfg.PaperTrading {
		t.log.Info("Not in paper trading mode")
		return
	}
	// Paper stats are sent through the stats
	t.tgBot.SendStats(t.stats)
}

func (t *Trader) resetPaperTrading() {
	if !t.cfg.PaperTrading {
		t.log.Info("Not in paper trading mode")
		return
	}

	t.mu.Lock()
	t.positions = make(map[string]*types.Position)
	t.trades = make([]*types.Trade, 0)
	t.stats = &types.BotStats{StartedAt: time.Now()}
	t.mu.Unlock()

	t.log.Info("[PAPER] Trading state reset")
}

func generatePositionID() string {
	return fmt.Sprintf("pos_%d", time.Now().UnixNano())
}

// showSmartWallets displays discovered profitable wallets via Telegram
func (t *Trader) showSmartWallets() {
	if t.smartScanner == nil {
		t.tgBot.SendReport("Smart wallet scanner not initialized.")
		return
	}

	wallets := t.smartScanner.GetTopWallets()
	if len(wallets) == 0 {
		t.tgBot.SendReport("🔍 *Smart Wallet Scanner*\n\n_Still scanning... no qualifying wallets yet._\n_Criteria: 65%+ win rate, 10+ trades, profitable_")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🧠 *TOP SMART WALLETS* (%d found)\n", len(wallets)))
	sb.WriteString("_65%%+ win rate · 10+ trades · profitable_\n\n")

	for i, addr := range wallets {
		if i >= 10 {
			break
		}
		stats := t.smartScanner.GetWalletStats(addr)
		short := addr[:8] + "..." + addr[len(addr)-4:]
		if stats != nil {
			sb.WriteString(fmt.Sprintf("#%d `%s`\n    WR: *%.0f%%* | PnL: *%.1f SOL* | Trades: %d\n",
				i+1, short, stats.WinRate*100, stats.TotalPnL, stats.TotalTrades))
		} else {
			sb.WriteString(fmt.Sprintf("#%d `%s`\n", i+1, short))
		}
	}
	sb.WriteString("\n_Use /copy\\_smart to start copy trading the top wallets_")
	t.tgBot.SendReport(sb.String())
}

// copySmartWallets adds top smart wallets to tracked wallets for copy trading
func (t *Trader) copySmartWallets() {
	if t.smartScanner == nil {
		t.log.Info("Smart wallet scanner not initialized")
		return
	}

	wallets := t.smartScanner.GetTopWallets()
	if len(wallets) == 0 {
		t.tgBot.SendReport("🔍 *Copy Smart*\n\n_Scanner hasn't found qualifying wallets yet._\n_Criteria: 65%+ win rate, 10+ trades, profitable_\n_Try again in a few minutes._")
		return
	}

	// Add up to 5 wallets — OnWalletAdded callback handles Telegram notification per wallet
	added := 0
	for i, addr := range wallets {
		if i >= 5 {
			break
		}
		if t.walletTracker.AddWallet(addr) {
			added++
		}
	}

	if added == 0 {
		t.tgBot.SendReport("✅ Already copy trading the top wallets — no changes needed.")
	} else {
		t.tgBot.SendReport(fmt.Sprintf("🐋 *Copy trading started* for *%d* top wallet(s).\nUse /wallets to see their status.", added))
	}
	t.log.WithField("count", added).Info("Added top smart wallets to copy trading")
}

// showTrackedWallets shows currently copy-traded wallets and their stats
func (t *Trader) showTrackedWallets() {
	if t.walletTracker == nil {
		t.tgBot.SendReport("Wallet tracker not initialized.")
		return
	}

	tracked := t.walletTracker.GetTrackedWallets()
	if len(tracked) == 0 {
		t.tgBot.SendReport("🐋 *Copy Trading*\n\n_No wallets currently tracked._\n\nUse:\n• `/copy_smart` — auto-add top discovered wallets\n• `/add_wallet <address>` — manually add a wallet\n• `/smart_wallets` — see discovered candidates")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🐋 *COPY TRADING* (%d wallets)\n\n", len(tracked)))

	for _, stats := range tracked {
		short := stats.Address[:8] + "..." + stats.Address[len(stats.Address)-4:]
		if stats.TotalTrades == 0 {
			age := time.Since(stats.AddedAt)
			sb.WriteString(fmt.Sprintf("👛 `%s`\n   _Watching... added %.0fm ago_\n\n", short, age.Minutes()))
		} else {
			wrEmoji := "🟡"
			if stats.WinRate >= 0.65 {
				wrEmoji = "🟢"
			} else if stats.WinRate < 0.50 {
				wrEmoji = "🔴"
			}
			pnlSign := "+"
			if stats.TotalPnL < 0 {
				pnlSign = ""
			}
			sb.WriteString(fmt.Sprintf("%s `%s`\n   WR: *%.0f%%* | PnL: *%s%.3f SOL* | Trades: %d\n\n",
				wrEmoji, short, stats.WinRate*100, pnlSign, stats.TotalPnL, stats.TotalTrades))
		}
	}

	sb.WriteString("_Underperforming wallets (WR < 60%%) auto-replaced each scan_")
	t.tgBot.SendReport(sb.String())
}

// showAnalytics displays analytics summary
func (t *Trader) showAnalytics() {
	summary := t.analytics.GetAnalyticsSummary()
	t.log.Info(summary)
	// Also send to Telegram if we add that method
}

// showPortfolio displays portfolio summary
func (t *Trader) showPortfolio() {
	summary := t.positionManager.GetPortfolioSummary()
	t.log.Info(summary)
}

// showPortfolioHeat shows current portfolio heat/risk
func (t *Trader) showPortfolioHeat() {
	heat := t.positionManager.GetPortfolioHeat()
	positions := t.positionManager.GetPositionCount()
	t.log.WithFields(logrus.Fields{
		"heat":      fmt.Sprintf("%.1f%%", heat*100),
		"positions": positions,
		"max_heat":  "50%",
	}).Info("Portfolio heat status")
}

// computePositionSize implements dynamic sizing:
// position = min(2% portfolio risk sizing, LP cap %) * sizeMultiplier
func (t *Trader) computePositionSize(ctx context.Context, safety *types.TokenSafety, sizeMultiplier float64) float64 {
	// Base risk sizing (e.g. 2% of portfolio)
	baseRiskSize := t.cfg.PortfolioSize * t.cfg.RiskPerTrade
	if baseRiskSize <= 0 {
		return 0
	}

	// LP cap sizing: max X% of LP, converted to SOL using current SOL price
	lpCapSize := baseRiskSize
	if safety != nil && safety.LiquidityUSD > 0 && !t.cfg.PaperTrading && t.solana != nil && t.cfg.MaxLpSharePercent > 0 {
		if solPrice, err := t.solana.GetSOLPrice(ctx); err == nil && solPrice > 0 {
			maxPositionUSD := safety.LiquidityUSD * (t.cfg.MaxLpSharePercent / 100.0)
			lpCapSize = maxPositionUSD / solPrice
		}
	}

	// Take the stricter of risk sizing and LP cap
	size := baseRiskSize
	if lpCapSize < size {
		size = lpCapSize
	}

	// Respect global max position cap if configured
	if t.cfg.MaxPositionSize > 0 && size > t.cfg.MaxPositionSize {
		size = t.cfg.MaxPositionSize
	}

	if size <= 0 {
		return 0
	}

	return size * sizeMultiplier
}

// updateDistribution ingests a wallet activity into rolling flow stats
func (t *Trader) updateDistribution(activity *types.WalletActivity) {
	if activity == nil || activity.TokenAddress == "" {
		return
	}

	window := t.distributionWindow
	if window <= 0 {
		// Fallback to 60s if not configured
		window = 60 * time.Second
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	stats, exists := t.distribution[activity.TokenAddress]
	if !exists {
		stats = &distributionStats{
			Events: make([]walletFlowEvent, 0, 32),
		}
		t.distribution[activity.TokenAddress] = stats
	}

	// Append new event
	stats.Events = append(stats.Events, walletFlowEvent{
		Time:   activity.Timestamp,
		Side:   activity.Action,
		Amount: activity.AmountSOL,
	})

	// Prune old events outside window
	cutoff := time.Now().Add(-window)
	pruneIdx := 0
	for i, ev := range stats.Events {
		if ev.Time.After(cutoff) {
			pruneIdx = i
			break
		}
	}
	if pruneIdx > 0 && pruneIdx < len(stats.Events) {
		stats.Events = stats.Events[pruneIdx:]
	}
}

// getDistributionScore returns net sell flow score [0,1] for a token
// 0 = only buys, 1 = only sells
func (t *Trader) getDistributionScore(tokenAddress string) float64 {
	window := t.distributionWindow
	if window <= 0 {
		window = 60 * time.Second
	}
	cutoff := time.Now().Add(-window)

	t.mu.RLock()
	stats, exists := t.distribution[tokenAddress]
	t.mu.RUnlock()
	if !exists {
		return 0
	}

	var buys, sells float64
	for _, ev := range stats.Events {
		if ev.Time.Before(cutoff) {
			continue
		}
		if ev.Amount <= 0 {
			continue
		}
		if ev.Side == "buy" {
			buys += ev.Amount
		} else if ev.Side == "sell" {
			sells += ev.Amount
		}
	}

	total := buys + sells
	if total == 0 {
		return 0
	}

	return sells / total
}

// isLpDropTooHigh records the latest LP value and returns true if the drop
// from the previous sample within the configured window exceeds the threshold.
func (t *Trader) isLpDropTooHigh(tokenAddress string, currentLiquidityUSD float64) bool {
	if currentLiquidityUSD <= 0 || t.cfg.MaxLpDropPercent <= 0 || t.cfg.LpDropWindowSeconds <= 0 {
		return false
	}

	window := time.Duration(t.cfg.LpDropWindowSeconds) * time.Second
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

	prev, exists := t.lpHistory[tokenAddress]
	t.lpHistory[tokenAddress] = lpSample{
		LiquidityUSD: currentLiquidityUSD,
		Time:         now,
	}

	if !exists {
		return false
	}

	// Only compare if previous sample is within the window
	if now.Sub(prev.Time) > window || prev.LiquidityUSD <= 0 {
		return false
	}

	drop := (prev.LiquidityUSD - currentLiquidityUSD) / prev.LiquidityUSD * 100
	return drop >= t.cfg.MaxLpDropPercent
}

// sanitizeTokenAddress cleans up token address input
func sanitizeTokenAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	addr = strings.Trim(addr, "<>[](){}")
	addr = strings.TrimPrefix(addr, "$")
	return addr
}

// checkToken performs safety analysis on a token and sends results
func (t *Trader) checkToken(ctx context.Context, tokenAddress string, args []string) {
	// Sanitize the token address
	tokenAddress = sanitizeTokenAddress(tokenAddress)

	source := "manual_check"
	if len(args) > 1 {
		source = args[1]
	}

	// Validate address length
	if len(tokenAddress) < 32 || len(tokenAddress) > 44 {
		t.log.WithField("address", tokenAddress).Warn("Invalid token address length")
		return
	}

	t.log.WithFields(logrus.Fields{
		"token":  tokenAddress,
		"source": source,
	}).Info("Manual token check requested")

	// Get token info
	var token *types.Token
	var err error

	if t.solana != nil {
		token, err = t.solana.GetTokenInfo(ctx, tokenAddress)
		if err != nil {
			t.log.WithError(err).Warn("Failed to get token info")
			token = &types.Token{
				Address:      tokenAddress,
				Symbol:       tokenAddress[:6],
				Name:         "Unknown Token",
				Decimals:     9,
				DiscoveredAt: time.Now(),
			}
		}
	} else {
		token = &types.Token{
			Address:      tokenAddress,
			Symbol:       tokenAddress[:6],
			Name:         "Unknown Token",
			Decimals:     9,
			DiscoveredAt: time.Now(),
		}
	}

	token.Source = source

	// Run full safety checks
	safetyResult := t.safetyChecker.CheckToken(ctx, token)

	// Check if tracked wallets hold this token
	if t.cfg.WalletTrackingEnabled {
		holdersCount, wallets := t.walletTracker.CheckWalletHoldsToken(ctx, tokenAddress)
		safetyResult.TrackedWalletsBought = holdersCount
		if holdersCount > 0 {
			safetyResult.SmartMoneyIn = true
			t.log.WithField("wallets", wallets).Info("Tracked wallets hold this token")
		}
	}

	// Create signal for display
	signal := &types.Signal{
		TokenAddress: tokenAddress,
		ChannelName:  source,
		ReceivedAt:   time.Now(),
	}

	// Send the analysis result
	if err := t.tgBot.SendSignalAlert(signal, safetyResult); err != nil {
		t.log.WithError(err).Warn("Failed to send check result")
	}

	t.log.WithFields(logrus.Fields{
		"token":   token.Symbol,
		"valid":   safetyResult.IsValid,
		"score":   safetyResult.Score,
		"reasons": safetyResult.Reasons,
	}).Info("Token check complete")
}
