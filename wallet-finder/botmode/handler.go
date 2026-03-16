package botmode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"wallet-finder/analyzer"
	"wallet-finder/api"
	"wallet-finder/config"
	"wallet-finder/models"
	"wallet-finder/output"
	"wallet-finder/scorer"
	"wallet-finder/telegram"
)

const helpText = `*SOL Wallet Finder — Commands*

/run — Scan Birdeye leaderboards and send today's top wallets here
/list — Show all wallets currently being copy-traded
/add <address> — Add a wallet to the copy-trade list
/remove <address> — Remove a wallet from the copy-trade list
/help — Show this message`

// Listen starts the Telegram polling loop and handles commands.
func Listen(cfg *config.Config, tg *telegram.Client) {
	fmt.Println("[BOT] Starting...")
	fmt.Printf("[BOT] Token set: %v\n", cfg.TelegramToken != "")
	fmt.Printf("[BOT] ChatID set: %v\n", cfg.TelegramChatID != "")

	if err := tg.SetMyCommands(); err != nil {
		fmt.Printf("[BOT] SetMyCommands error: %v\n", err)
	} else {
		fmt.Println("[BOT] Commands registered OK")
	}

	// Notify user the bot is online
	if err := tg.Send("🟢 SOL Wallet Finder bot is online\\! Send /help for commands\\."); err != nil {
		fmt.Printf("[BOT] Failed to send startup message: %v\n", err)
	} else {
		fmt.Println("[BOT] Startup message sent to Telegram")
	}

	fmt.Println("[BOT] Polling for commands... (Ctrl+C to stop)")

	offset := 0
	for {
		updates, err := tg.GetUpdates(offset)
		if err != nil {
			fmt.Printf("[BOT] poll error: %v — retrying in 5s\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, upd := range updates {
			offset = upd.UpdateID + 1
			text := strings.TrimSpace(upd.Message.Text)
			chatID := upd.Message.Chat.ID
			if text == "" {
				continue
			}
			fmt.Printf("[BOT] received from %d: %s\n", chatID, text)
			handle(cfg, tg, chatID, text)
		}
	}
}

func handle(cfg *config.Config, tg *telegram.Client, chatID int64, text string) {
	parts := strings.Fields(text)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help", "/start":
		_ = tg.Reply(chatID, helpText)

	case "/list":
		wallets := loadWallets(cfg.BotWalletsFile)
		if len(wallets) == 0 {
			_ = tg.Reply(chatID, "No wallets are currently being copy-traded.")
			return
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("*Copy-trading %d wallets:*\n\n", len(wallets)))
		i := 1
		for addr, label := range wallets {
			if label != "" {
				sb.WriteString(fmt.Sprintf("%d. `%s`\n   _%s_\n", i, addr, label))
			} else {
				sb.WriteString(fmt.Sprintf("%d. `%s`\n", i, addr))
			}
			i++
		}
		_ = tg.Reply(chatID, sb.String())

	case "/add":
		if len(parts) < 2 {
			_ = tg.Reply(chatID, "Usage: /add <wallet_address>")
			return
		}
		addr := parts[1]
		wallets := loadWallets(cfg.BotWalletsFile)
		if _, exists := wallets[addr]; exists {
			_ = tg.Reply(chatID, fmt.Sprintf("⚠️ Already tracking `%s`", addr))
			return
		}
		wallets[addr] = "manually added via Telegram"
		if err := saveWallets(cfg.BotWalletsFile, wallets); err != nil {
			_ = tg.Reply(chatID, fmt.Sprintf("❌ Failed to save: %v", err))
			return
		}
		_ = tg.Reply(chatID, fmt.Sprintf("✅ Added `%s` to copy-trade list.", addr))

	case "/remove":
		if len(parts) < 2 {
			_ = tg.Reply(chatID, "Usage: /remove <wallet_address>")
			return
		}
		addr := parts[1]
		wallets := loadWallets(cfg.BotWalletsFile)
		if _, exists := wallets[addr]; !exists {
			_ = tg.Reply(chatID, fmt.Sprintf("⚠️ `%s` is not in the list.", addr))
			return
		}
		delete(wallets, addr)
		if err := saveWallets(cfg.BotWalletsFile, wallets); err != nil {
			_ = tg.Reply(chatID, fmt.Sprintf("❌ Failed to save: %v", err))
			return
		}
		_ = tg.Reply(chatID, fmt.Sprintf("🗑 Removed `%s` from copy-trade list.", addr))

	case "/run":
		_ = tg.Reply(chatID, "🔍 Starting wallet scan... this takes a few minutes.")
		go runScan(cfg, tg, chatID)

	default:
		_ = tg.Reply(chatID, "Unknown command. Send /help for the list.")
	}
}

func runScan(cfg *config.Config, tg *telegram.Client, chatID int64) {
	ctx := context.Background()
	birdeye := api.NewBirdeye(cfg.BirdeyeAPIKey)
	helius := api.NewHelius(cfg.HeliusAPIKey)

	appearCount := make(map[string]int)
	bestData := make(map[string]api.BirdeyeCandidate)

	for _, period := range api.AllPeriods {
		candidates, err := birdeye.TopTraders(ctx, period, cfg.DiscoveryBatchSize)
		if err != nil {
			continue
		}
		for _, c := range candidates {
			appearCount[c.Address]++
			prev, exists := bestData[c.Address]
			if !exists || c.TradeCount > prev.TradeCount {
				bestData[c.Address] = c
			}
		}
		time.Sleep(1 * time.Second)
	}

	var filtered []api.BirdeyeCandidate
	for addr, c := range bestData {
		if c.PnL <= 0 || c.TradeCount < cfg.MinTrades {
			continue
		}
		c.Address = addr
		c.PeriodCount = appearCount[addr]
		filtered = append(filtered, c)
	}

	if len(filtered) == 0 {
		_ = tg.Reply(chatID, "❌ No candidates found from leaderboards.")
		return
	}

	var analyses []*models.WalletAnalysis
	for _, c := range filtered {
		txs, err := helius.GetSwapTransactions(ctx, c.Address, cfg.HeliusTxLimit)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		wa := analyzer.AnalyzeHistory(c.Address, txs, c)

		if wa.BirdeyeWinRate < cfg.MinWinRate { continue }
		if wa.WinCount < cfg.MinWinCount { continue }
		if wa.WinDays < cfg.MinWinDays { continue }
		if cfg.MaxTopWinPct > 0 && wa.TopWinPct > cfg.MaxTopWinPct { continue }
		if wa.HistoryDays < cfg.MinHistoryDays { continue }
		if cfg.MaxActiveAgoDays > 0 && wa.DaysSinceActive > cfg.MaxActiveAgoDays { continue }

		wa.Score = scorer.Score(wa)
		analyses = append(analyses, wa)
		time.Sleep(200 * time.Millisecond)
	}

	if len(analyses) == 0 {
		_ = tg.Reply(chatID, "❌ No wallets passed all filters. Try again later or relax filters in .env")
		return
	}

	ranked := scorer.Rank(analyses, cfg.TopN)
	_ = output.SaveJSON(ranked, cfg.OutputFile)

	date := time.Now().Format("2006-01-02")
	msg := output.FormatTelegram(ranked, date)
	_ = tg.SendChunkedTo(chatID, msg)
}

func loadWallets(path string) map[string]string {
	m := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	_ = json.Unmarshal(data, &m)
	return m
}

func saveWallets(path string, wallets map[string]string) error {
	data, err := json.MarshalIndent(wallets, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
