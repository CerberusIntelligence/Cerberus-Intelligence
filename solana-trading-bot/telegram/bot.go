package telegram

import (
	"fmt"
	"strings"
	"time"
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"solana-trading-bot/config"
	"solana-trading-bot/types"

	log "github.com/sirupsen/logrus"
)

// EngineAPI is the subset of engine.Engine used by the bot.
type EngineAPI interface {
	GetStatus() (float64, map[string]*types.Position, []types.Trade)
	GetSessionStartedAt() time.Time
	AlertCh() <-chan string
	Pause()
	Resume()
	IsPaused() bool
	ClosePosition(address string) error
	SetStopLoss(pct float64)
	GetConfig() *config.Config
	InjectSignal(address, source string)
	AddTrackedWallet(address string)
	GetTrackedWallets() []string
}

// Bot handles Telegram bot API interactions (alerts + control commands).
type Bot struct {
	cfg    *config.Config
	api    *tgbotapi.BotAPI
	engine EngineAPI
}

func NewBot(cfg *config.Config, eng EngineAPI) *Bot {
	return &Bot{cfg: cfg, engine: eng}
}

// Start runs the bot. Call in a goroutine.
func (b *Bot) Start(ctx context.Context) {
	if b.cfg.TelegramBotToken == "" {
		log.Warn("No bot token configured — Telegram alerts disabled")
		return
	}

	api, err := tgbotapi.NewBotAPI(b.cfg.TelegramBotToken)
	if err != nil {
		log.WithError(err).Error("Telegram bot init failed")
		return
	}
	b.api = api
	log.WithField("username", "@"+api.Self.UserName).Info("Telegram bot connected")

	mode := "PAPER"
	if !b.cfg.PaperTrading {
		mode = "LIVE"
	}
	b.send(fmt.Sprintf("🤖 *Alpha Bot Online*\nMode: `%s` | Balance: `%.4f SOL`\nType /help for commands", mode, b.cfg.PaperBalance))

	go b.processAlerts(ctx)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			if update.Message == nil {
				continue
			}
			// Only accept from authorised chat
			if b.cfg.TelegramChatID != 0 && update.Message.Chat.ID != b.cfg.TelegramChatID {
				continue
			}
			if update.Message.IsCommand() || strings.HasPrefix(update.Message.Text, "/") {
				b.handleCommand(update.Message)
			}
		}
	}
}

func (b *Bot) processAlerts(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-b.engine.AlertCh():
			b.send(msg)
		}
	}
}

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	parts := strings.Fields(msg.Text)
	if len(parts) == 0 {
		return
	}
	cmd := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	// Strip @botname suffix if present
	if i := strings.Index(cmd, "@"); i != -1 {
		cmd = cmd[:i]
	}

	switch cmd {
	case "start", "help":
		b.cmdHelp()
	case "report":
		b.cmdReport()
	case "status":
		b.cmdStatus()
	case "positions", "pos":
		b.cmdPositions()
	case "history", "hist":
		b.cmdHistory()
	case "params":
		b.cmdParams()
	case "pause":
		b.engine.Pause()
		b.send("⏸ Trading *paused*")
	case "resume":
		b.engine.Resume()
		b.send("▶️ Trading *resumed*")
	case "close":
		if len(parts) < 2 {
			b.send("Usage: `/close <address>`")
			return
		}
		if err := b.engine.ClosePosition(parts[1]); err != nil {
			b.send("❌ " + err.Error())
		} else {
			b.send("✅ Closing position...")
		}
	case "setsl":
		if len(parts) < 2 {
			b.send("Usage: `/setsl 35` (percent)")
			return
		}
		var pct float64
		fmt.Sscanf(parts[1], "%f", &pct)
		b.engine.SetStopLoss(pct / 100)
		b.send(fmt.Sprintf("✅ Stop loss set to `%.0f%%`", pct))
	case "buy", "check":
		if len(parts) < 2 {
			b.send("Usage: `/buy <address>` — manually inject a signal")
			return
		}
		b.engine.InjectSignal(parts[1], "manual")
		b.send("🔍 Signal injected — validating `" + parts[1][:8] + "...`")
	case "addwallet":
		if len(parts) < 2 {
			b.send("Usage: `/addwallet <wallet_address>`")
			return
		}
		addr := strings.TrimSpace(parts[1])
		if len(addr) < 32 {
			b.send("❌ Invalid wallet address")
			return
		}
		b.engine.AddTrackedWallet(addr)
		b.send("✅ Now tracking wallet `" + addr[:8] + "...` — will copy their buys")
	case "wallets":
		wallets := b.engine.GetTrackedWallets()
		if len(wallets) == 0 {
			b.send("👛 No wallets tracked yet\n\nUse `/addwallet <address>` to add one")
			return
		}
		msg := fmt.Sprintf("👛 *Tracked Wallets* (%d)\n\n", len(wallets))
		for _, w := range wallets {
			msg += "• `" + w + "`\n"
		}
		b.send(msg)
	default:
		b.send("Unknown command. Use /help")
	}
}

func (b *Bot) cmdHelp() {
	b.send(`🤖 *Mirror Bot Commands*

/status — balance, PnL, win rate
/report — today's P&L by wallet
/positions — open positions
/history — last 10 trades
/params — current settings

/pause — pause auto-trading
/resume — resume auto-trading
/close <addr> — manually close position
/buy <addr> — manually inject signal
/setsl 35 — set stop loss %
/addwallet <addr> — copy-trade a wallet
/wallets — list tracked wallets`)
}

func (b *Bot) cmdStatus() {
	bal, positions, history := b.engine.GetStatus()
	sessionStart := b.engine.GetSessionStartedAt()

	totalPnL := 0.0
	wins, losses := 0, 0
	for _, t := range history {
		if t.ClosedAt.Before(sessionStart) {
			continue
		}
		totalPnL += t.PnLSOL
		if t.PnLSOL > 0 {
			wins++
		} else {
			losses++
		}
	}

	winRate := 0.0
	if wins+losses > 0 {
		winRate = float64(wins) / float64(wins+losses) * 100
	}

	status := "▶️ Running"
	if b.engine.IsPaused() {
		status = "⏸ Paused"
	}

	mode := "PAPER"
	if !b.cfg.PaperTrading {
		mode = "LIVE"
	}

	b.send(fmt.Sprintf(
		"📊 *Status*\n"+
			"Status: %s | Mode: `%s`\n\n"+
			"💰 Balance: `%.4f SOL`\n"+
			"📈 Total PnL: `%.4f SOL`\n"+
			"🎯 Win Rate: `%.1f%%` (%d wins / %d losses)\n"+
			"📂 Open: `%d` positions",
		status, mode,
		bal, totalPnL,
		winRate, wins, losses,
		len(positions),
	))
}

func (b *Bot) cmdReport() {
	_, _, history := b.engine.GetStatus()
	wallets := b.engine.GetTrackedWallets()

	// Filter to today's closed trades
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	type walletStats struct {
		wins, losses int
		pnl          float64
	}
	stats := make(map[string]*walletStats)
	totalWins, totalLosses := 0, 0
	totalPnL := 0.0

	for _, t := range history {
		if t.ClosedAt.Before(today) {
			continue
		}
		src := t.Source
		if stats[src] == nil {
			stats[src] = &walletStats{}
		}
		stats[src].pnl += t.PnLSOL
		totalPnL += t.PnLSOL
		if t.PnLSOL > 0 {
			stats[src].wins++
			totalWins++
		} else if t.PnLSOL < 0 {
			stats[src].losses++
			totalLosses++
		}
	}

	totalEmoji := "🟢"
	if totalPnL < 0 {
		totalEmoji = "🔴"
	}

	msg := fmt.Sprintf(
		"📊 *Daily Report* (%s)\n%s Total: `%+.4f SOL` | ✅ %dW / ❌ %dL\n\n👛 *By Wallet*\n",
		now.Format("Jan 2 15:04"),
		totalEmoji, totalPnL, totalWins, totalLosses,
	)

	// Build short-prefix → full address map
	addrMap := make(map[string]string)
	for _, addr := range wallets {
		if len(addr) >= 8 {
			addrMap["wallet:"+addr[:8]] = addr
		}
	}

	// List wallets that traded today
	shown := make(map[string]bool)
	for src, s := range stats {
		shown[src] = true
		emoji := "🟢"
		if s.pnl < 0 {
			emoji = "🔴"
		} else if s.pnl == 0 {
			emoji = "⚪"
		}
		label := src
		if full, ok := addrMap[src]; ok {
			label = full[:12] + "..."
		}
		msg += fmt.Sprintf("%s `%s` — `%+.4f SOL` (%dW/%dL)\n", emoji, label, s.pnl, s.wins, s.losses)
	}

	// List wallets with no trades today
	for _, addr := range wallets {
		src := "wallet:" + addr[:8]
		if !shown[src] {
			msg += fmt.Sprintf("⚪ `%s...` — no trades today\n", addr[:12])
		}
	}

	b.send(msg)
}

func (b *Bot) cmdPositions() {
	_, positions, _ := b.engine.GetStatus()

	if len(positions) == 0 {
		b.send("📭 No open positions")
		return
	}

	msg := fmt.Sprintf("📂 *Open Positions* (%d)\n\n", len(positions))
	for _, pos := range positions {
		pnlPct := 0.0
		if pos.EntryPrice > 0 {
			pnlPct = (pos.CurrentPrice - pos.EntryPrice) / pos.EntryPrice * 100
		}
		emoji := "🔴"
		if pnlPct > 0 {
			emoji = "🟢"
		}
		msg += fmt.Sprintf(
			"%s `%s` — `%s`\n"+
				"   Entry: `%.8f` → Now: `%.8f`\n"+
				"   PnL: `%.1f%%` | Held: `%s`\n\n",
			emoji, pos.Symbol, pos.Address[:12]+"...",
			pos.EntryPrice, pos.CurrentPrice,
			pnlPct, time.Since(pos.OpenedAt).Round(time.Second),
		)
	}
	b.send(msg)
}

func (b *Bot) cmdHistory() {
	_, _, history := b.engine.GetStatus()
	sessionStart := b.engine.GetSessionStartedAt()

	// Filter to session-only trades
	var sessionHistory []types.Trade
	for _, t := range history {
		if !t.ClosedAt.Before(sessionStart) {
			sessionHistory = append(sessionHistory, t)
		}
	}

	if len(sessionHistory) == 0 {
		b.send("📭 No trades this session yet")
		return
	}

	start := 0
	if len(sessionHistory) > 10 {
		start = len(sessionHistory) - 10
	}

	msg := "📜 *Last Trades (This Session)*\n\n"
	for _, t := range sessionHistory[start:] {
		emoji := "🔴"
		if t.PnLSOL > 0 {
			emoji = "🟢"
		}
		msg += fmt.Sprintf(
			"%s `%s` | `%.4f SOL (%.1f%%)` | `%s`\n",
			emoji, t.Symbol, t.PnLSOL, t.PnLPct, t.Reason,
		)
	}
	b.send(msg)
}

func (b *Bot) cmdParams() {
	cfg := b.engine.GetConfig()
	b.send(fmt.Sprintf(
		"⚙️ *Parameters*\n\n"+
			"Trade Amount: `%.4f SOL`\n"+
			"Max Positions: `%d`\n"+
			"Stop Loss: `%.0f%%`\n"+
			"Trailing Stop: `%.0f%%`\n"+
			"Take Profit: `%.1fx / %.1fx / %.1fx`\n"+
			"TP Sell %%: `%.0f%% / %.0f%% / %.0f%%`\n"+
			"Timeout: `%d min`\n"+
			"Min Liquidity: `$%.0f`\n"+
			"Max Trades/hr: `%d`",
		cfg.TradeAmountSOL, cfg.MaxPositions,
		cfg.StopLossPct*100, cfg.TrailingStopPct*100,
		cfg.TakeProfit1x, cfg.TakeProfit2x, cfg.TakeProfit3x,
		cfg.TP1Pct*100, cfg.TP2Pct*100, cfg.TP3Pct*100,
		cfg.TimeoutMinutes, cfg.MinLiquidityUSD,
		cfg.MaxTradesPerHour,
	))
}

func (b *Bot) send(text string) {
	if b.api == nil || b.cfg.TelegramChatID == 0 {
		return
	}
	msg := tgbotapi.NewMessage(b.cfg.TelegramChatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	if _, err := b.api.Send(msg); err != nil {
		log.WithError(err).Warn("Telegram send failed")
	}
}
