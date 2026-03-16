package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wallet-finder/analyzer"
	"wallet-finder/api"
	"wallet-finder/botmode"
	"wallet-finder/config"
	"wallet-finder/discovery"
	"wallet-finder/models"
	"wallet-finder/output"
	"wallet-finder/scorer"
	"wallet-finder/telegram"
)

func main() {
	minWinRate := flag.Float64("min-wr", 0, "Override MIN_WIN_RATE (e.g. 0.60)")
	minTrades := flag.Int("min-trades", 0, "Override MIN_TRADES (e.g. 50)")
	minHistory := flag.Int("min-history", 0, "Override MIN_HISTORY_DAYS (e.g. 30)")
	topN := flag.Int("top", 0, "Override TOP_N (e.g. 100)")
	exportBot := flag.Bool("export-bot", false, "Export top wallets to bot's tracked_wallets.json")
	botMode := flag.Bool("bot", false, "Run as Telegram bot — listen for commands")
	noFilter := flag.Bool("no-filter", false, "Skip all filters and save every analyzed wallet")
	huntMode := flag.Bool("hunt", false, "Use token-hunter mode: find smart money from pumped tokens via DexScreener")
	gmgnMode := flag.Bool("gmgn", false, "Use GMGN.ai leaderboard scraper to find high-quality wallets")
	flag.Parse()

	cfg := config.Load()
	if *minWinRate > 0 {
		cfg.MinWinRate = *minWinRate
	}
	if *minTrades > 0 {
		cfg.MinTrades = *minTrades
	}
	if *minHistory > 0 {
		cfg.MinHistoryDays = *minHistory
	}
	if *topN > 0 {
		cfg.TopN = *topN
	}
	if *exportBot {
		cfg.ExportForBot = true
	}

	if cfg.BirdeyeAPIKey == "" {
		fmt.Fprintln(os.Stderr, "[✗] BIRDEYE_API_KEY not set in .env")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("\n[!] Interrupted — saving partial results...")
		cancel()
	}()

	birdeye := api.NewBirdeye(cfg.BirdeyeAPIKey)
	helius := api.NewHelius(cfg.HeliusAPIKey)

	// Set up Telegram if configured
	var tg *telegram.Client
	if cfg.TelegramToken != "" && cfg.TelegramChatID != "" {
		tg = telegram.NewClient(cfg.TelegramToken, cfg.TelegramChatID)
		fmt.Println("[✓] Telegram notifications enabled")
	}

	// Bot mode: listen for Telegram commands and handle them interactively
	if *botMode {
		if tg == nil {
			fmt.Fprintln(os.Stderr, "[✗] TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must be set in .env for bot mode")
			os.Exit(1)
		}
		botmode.Listen(cfg, tg)
		return
	}

	// Hunt mode: discover wallets from pumped tokens instead of leaderboards
	if *huntMode {
		fmt.Println("╔══════════════════════════════════════════════════╗")
		fmt.Println("║     SMART MONEY HUNTER  (DexScreener mode)       ║")
		fmt.Println("╚══════════════════════════════════════════════════╝")
		dex := api.NewDexscreener()
		fmt.Println("[1/3] Finding pumped Solana tokens from DexScreener...")
		tokens, err := dex.FindPumpedTokens(ctx, 50.0, 10000.0)
		if err != nil || len(tokens) == 0 {
			fmt.Printf("[!] DexScreener failed or no pumped tokens found: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("    Found %d pumped tokens\n\n", len(tokens))

		fmt.Println("[2/3] Scanning liquidity pools for early profitable buyers...")
		candidates := discovery.FindSmartMoney(ctx, helius, tokens, 200)

		if len(candidates) == 0 {
			fmt.Println("[!] No smart money wallets found across pumped tokens.")
			os.Exit(0)
		}

		fmt.Printf("\n[3/3] Deep analysis of %d smart money candidates...\n", len(candidates))
		var analyses []*models.WalletAnalysis
		passed, skipped := 0, 0
		for i, addr := range candidates {
			select {
			case <-ctx.Done():
				goto huntScore
			default:
			}
			fmt.Printf("    [%d/%d] %s", i+1, len(candidates), addr[:8]+"...")
			txs, err := helius.GetSwapTransactions(ctx, addr, cfg.HeliusTxLimit)
			if err != nil {
				fmt.Printf("  [err: %v]\n", err)
				skipped++
				continue
			}
			wa := analyzer.AnalyzeHistory(addr, txs, api.BirdeyeCandidate{Address: addr})
			fmt.Printf("  wr=%.1f%%  wins=%d  wdays=%d  pnl=%.2f◎\n",
				wa.BirdeyeWinRate*100, wa.WinCount, wa.WinDays, wa.TotalPnLSOL)
			if wa.BirdeyeWinRate < cfg.MinWinRate || wa.WinCount < cfg.MinWinCount || wa.TotalPnLSOL < cfg.MinHeliusPnLSOL {
				skipped++
				continue
			}
			wa.Score = scorer.Score(wa)
			analyses = append(analyses, wa)
			passed++
			time.Sleep(200 * time.Millisecond)
		}
	huntScore:
		fmt.Printf("\n[✓] %d passed  |  %d skipped\n", passed, skipped)
		if len(analyses) == 0 {
			fmt.Println("[!] No wallets survived filters.")
			os.Exit(0)
		}
		ranked := scorer.Rank(analyses, cfg.TopN)
		output.PrintTable(ranked)
		output.PrintSummary(ranked)
		if err := output.SaveJSON(ranked, cfg.OutputFile); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Could not save: %v\n", err)
		}
		if tg != nil {
			date := time.Now().Format("2006-01-02")
			_ = tg.SendChunked(output.FormatTelegram(ranked, date))
		}
		return
	}

	// GMGN mode: scrape GMGN.ai leaderboard for high-quality wallets with real history
	if *gmgnMode {
		fmt.Println("╔══════════════════════════════════════════════════╗")
		fmt.Println("║     GMGN.ai SMART WALLET SCRAPER                 ║")
		fmt.Println("╚══════════════════════════════════════════════════╝")
		gmgn := api.NewGMGN()

		seen := make(map[string]bool)
		var gmgnCandidates []api.GMGNWallet

		cancelled := false
		for _, period := range api.GMGNPeriods {
			if cancelled {
				break
			}
			select {
			case <-ctx.Done():
				cancelled = true
				continue
			default:
			}
			fmt.Printf("[→] Fetching GMGN %s leaderboard (top 500 by PnL)...\n", period)
			wallets, err := gmgn.TopWalletsPaged(ctx, period, "pnl", 500)
			if err != nil {
				fmt.Printf("    [!] %s failed: %v\n", period, err)
				continue
			}
			fmt.Printf("    Got %d wallets from %s\n", len(wallets), period)
			added := 0
			for _, w := range wallets {
				if w.Address == "" || seen[w.Address] {
					continue
				}
				seen[w.Address] = true
				gmgnCandidates = append(gmgnCandidates, w)
				added++
			}
			fmt.Printf("    %d new unique wallets added\n", added)
			time.Sleep(1 * time.Second)
		}

		// Also fetch smart money tagged wallets
		if !cancelled {
			fmt.Println("[→] Fetching GMGN smart_degen tagged wallets...")
			smartWallets, err := gmgn.SmartMoneyWallets(ctx, "30d", 100)
			if err != nil {
				fmt.Printf("    [!] Smart money fetch failed: %v\n", err)
			} else {
				added := 0
				for _, w := range smartWallets {
					if w.Address == "" || seen[w.Address] {
						continue
					}
					seen[w.Address] = true
					gmgnCandidates = append(gmgnCandidates, w)
					added++
				}
				fmt.Printf("    %d additional smart_degen wallets\n", added)
			}
		}

		fmt.Printf("\n[1/2] Pre-filtering %d GMGN candidates...\n", len(gmgnCandidates))

		// Pre-filter by GMGN's own data before burning Helius API calls
		var gmgnFiltered []api.GMGNWallet
		now := time.Now().Unix()
		for _, w := range gmgnCandidates {
			// Must have positive 30d realized profit above threshold
			pnl30d := w.RealizedProfit30d
			if pnl30d <= 0 {
				pnl30d = w.RealizedProfit7d
			}
			if pnl30d < cfg.MinPeriodPnLUSD {
				continue
			}
			// Win rate pre-screen from GMGN data — use 50% floor so we don't
			// discard wallets before Helius computes the real win rate.
			wr := w.Winrate30d
			if wr == 0 {
				wr = w.Winrate7d
			}
			if wr < 0.50 {
				continue
			}
			// Must have been active recently
			if cfg.MaxActiveAgoDays > 0 && w.LastActiveTime > 0 {
				daysSince := int((now - w.LastActiveTime) / 86400)
				if daysSince > cfg.MaxActiveAgoDays*3 { // 3x grace for GMGN timestamps
					continue
				}
			}
			// Must have enough trades
			trades := w.Buy30d + w.Sell30d
			if trades == 0 {
				trades = w.Buy + w.Sell
			}
			if trades < cfg.MinTrades {
				continue
			}
			// Must be a holder, not a flipper — GMGN avg hold must exceed minimum
			if cfg.MinHoldSeconds > 0 {
				hold := w.AvgHoldingPeriod30d
				if hold == 0 {
					hold = w.AvgHoldingPeriod7d
				}
				if hold > 0 && hold < float64(cfg.MinHoldSeconds) {
					continue
				}
			}
			gmgnFiltered = append(gmgnFiltered, w)
		}
		fmt.Printf("    After pre-filter: %d candidates\n\n", len(gmgnFiltered))

		if len(gmgnFiltered) == 0 {
			fmt.Println("[!] No candidates passed GMGN pre-filter. Try lowering thresholds.")
			if len(gmgnCandidates) > 0 {
				fmt.Printf("    Top GMGN wallet: %s  30dPnL=$%.0f  wr=%.1f%%\n",
					gmgnCandidates[0].Address, gmgnCandidates[0].RealizedProfit30d, gmgnCandidates[0].Winrate30d*100)
			}
			os.Exit(0)
		}

		fmt.Printf("[2/2] Deep Helius analysis of %d candidates...\n", len(gmgnFiltered))
		var analyses []*models.WalletAnalysis
		passed, skipped := 0, 0

		for i, gw := range gmgnFiltered {
			select {
			case <-ctx.Done():
				fmt.Println("\n[!] Scan cancelled — saving partial results.")
				goto gmgnScore
			default:
			}

			fmt.Printf("    [%d/%d] %s  30dPnL=$%.0f  wr=%.1f%%",
				i+1, len(gmgnFiltered), gw.Address[:8]+"...",
				gw.RealizedProfit30d, gw.Winrate30d*100)

			txs, err := helius.GetSwapTransactions(ctx, gw.Address, cfg.HeliusTxLimit)
			if err != nil {
				fmt.Printf("  [helius err: %v]\n", err)
				skipped++
				time.Sleep(500 * time.Millisecond)
				continue
			}

			// Build a BirdeyeCandidate from GMGN data so analyzer works
			bc := api.BirdeyeCandidate{
				Address:    gw.Address,
				PnL:        gw.RealizedProfit30d,
				TradeCount: gw.Buy30d + gw.Sell30d,
			}
			if bc.PnL == 0 {
				bc.PnL = gw.RealizedProfit7d
			}
			if bc.TradeCount == 0 {
				bc.TradeCount = gw.Buy + gw.Sell
			}

			wa := analyzer.AnalyzeHistory(gw.Address, txs, bc)
			holdStr := "n/a"
			if wa.AvgHoldSeconds > 0 {
				holdStr = fmt.Sprintf("%.0fm", wa.AvgHoldSeconds/60)
			}
			fmt.Printf("  swaps=%d  wr=%.1f%%  wins=%d  wdays=%d  hold=%s  pnl=%.2f◎",
				wa.SwapCount, wa.BirdeyeWinRate*100, wa.WinCount, wa.WinDays, holdStr, wa.TotalPnLSOL)

			if !*noFilter {
				if bc.PnL < cfg.MinPeriodPnLUSD {
					fmt.Printf("  [skip: pnl $%.0f < $%.0f]\n", bc.PnL, cfg.MinPeriodPnLUSD)
					skipped++
					continue
				}
				if wa.TotalPnLSOL < cfg.MinHeliusPnLSOL {
					fmt.Printf("  [skip: helius pnl %.1f◎ < %.1f◎]\n", wa.TotalPnLSOL, cfg.MinHeliusPnLSOL)
					skipped++
					continue
				}
				if wa.BirdeyeWinRate < cfg.MinWinRate {
					fmt.Printf("  [skip: wr %.1f%% < %.0f%%]\n", wa.BirdeyeWinRate*100, cfg.MinWinRate*100)
					skipped++
					continue
				}
				if wa.WinCount < cfg.MinWinCount {
					fmt.Printf("  [skip: only %d wins]\n", wa.WinCount)
					skipped++
					continue
				}
				if wa.WinDays < cfg.MinWinDays {
					fmt.Printf("  [skip: wins on only %d day(s)]\n", wa.WinDays)
					skipped++
					continue
				}
				if cfg.MaxTopWinPct > 0 && wa.TopWinPct > cfg.MaxTopWinPct {
					fmt.Printf("  [skip: scraper — top win = %.0f%%]\n", wa.TopWinPct*100)
					skipped++
					continue
				}
				if wa.HistoryDays < cfg.MinHistoryDays {
					fmt.Printf("  [skip: hist %dd < %dd]\n", wa.HistoryDays, cfg.MinHistoryDays)
					skipped++
					continue
				}
				if cfg.MaxActiveAgoDays > 0 && wa.DaysSinceActive > cfg.MaxActiveAgoDays {
					fmt.Printf("  [skip: idle %dd > %dd]\n", wa.DaysSinceActive, cfg.MaxActiveAgoDays)
					skipped++
					continue
				}
			}

			wa.Score = scorer.Score(wa)
			analyses = append(analyses, wa)
			passed++
			fmt.Printf("  → score=%.2f ✓\n", wa.Score)
			time.Sleep(200 * time.Millisecond)
		}

	gmgnScore:
		fmt.Printf("\n[✓] %d passed  |  %d skipped\n", passed, skipped)
		if len(analyses) == 0 {
			fmt.Println("[!] No wallets survived all filters.")
			fmt.Println("    Try lowering MIN_WIN_RATE or MIN_PERIOD_PNL_USD in .env")
			os.Exit(0)
		}
		ranked := scorer.Rank(analyses, cfg.TopN)
		output.PrintTable(ranked)
		output.PrintSummary(ranked)
		if err := output.SaveJSON(ranked, cfg.OutputFile); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Could not save: %v\n", err)
		}
		if tg != nil {
			date := time.Now().Format("2006-01-02")
			_ = tg.SendChunked(output.FormatTelegram(ranked, date))
		}
		if cfg.ExportForBot {
			top20 := scorer.RankByPnL(analyses, 20)
			if err := output.ExportForBot(top20, cfg.BotWalletsFile); err != nil {
				fmt.Fprintf(os.Stderr, "[!] Could not export to bot file: %v\n", err)
			}
		}
		return
	}

	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║        SOLANA WALLET FINDER  (Go edition)        ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Printf("  Filters: win_rate ≥ %.0f%%  |  trades ≥ %d  |  history ≥ %dd  |  active within %dd\n\n",
		cfg.MinWinRate*100, cfg.MinTrades, cfg.MinHistoryDays, cfg.MaxActiveAgoDays)

	// ── PHASE 1: Discover candidates via Birdeye leaderboards ───────────────
	fmt.Printf("[1/3] Fetching top %d traders from Birdeye (today + 1W)...\n", cfg.DiscoveryBatchSize)

	appearCount := make(map[string]int)
	bestData := make(map[string]api.BirdeyeCandidate)

	for _, period := range api.AllPeriods {
		select {
		case <-ctx.Done():
			goto prefilter
		default:
		}

		candidates, err := birdeye.TopTraders(ctx, period, cfg.DiscoveryBatchSize)
		if err != nil {
			fmt.Printf("    [!] %s leaderboard failed: %v (skipping)\n", period, err)
			time.Sleep(1 * time.Second)
			continue
		}
		fmt.Printf("    %s leaderboard: %d wallets\n", period, len(candidates))

		for _, c := range candidates {
			appearCount[c.Address]++
			prev, exists := bestData[c.Address]
			if !exists || c.TradeCount > prev.TradeCount {
				bestData[c.Address] = c
			}
		}
		time.Sleep(1 * time.Second) // avoid rate limiting between leaderboard calls
	}

prefilter:
	fmt.Printf("    Total unique wallets discovered: %d\n", len(bestData))

	// ── PHASE 2: Pre-filter by Birdeye PnL + trade count ────────────────────
	fmt.Printf("\n[2/3] Pre-filtering (positive PnL, trades ≥ %d)...\n", cfg.MinTrades)

	var filtered []api.BirdeyeCandidate
	for addr, c := range bestData {
		if c.PnL <= 0 {
			continue
		}
		if c.TradeCount < cfg.MinTrades {
			continue
		}
		c.Address = addr
		c.PeriodCount = appearCount[addr]
		filtered = append(filtered, c)
	}

	// Sort: multi-period first, then by PnL descending
	for i := 1; i < len(filtered); i++ {
		for j := i; j > 0; j-- {
			a, b := filtered[j-1], filtered[j]
			if b.PeriodCount > a.PeriodCount || (b.PeriodCount == a.PeriodCount && b.PnL > a.PnL) {
				filtered[j-1], filtered[j] = filtered[j], filtered[j-1]
			} else {
				break
			}
		}
	}

	multiPeriod := 0
	for _, c := range filtered {
		if c.PeriodCount >= 2 {
			multiPeriod++
		}
	}
	fmt.Printf("    After pre-filter: %d candidates (%d in both periods)\n", len(filtered), multiPeriod)

	if len(filtered) == 0 {
		fmt.Println("[!] No candidates found. The leaderboards may be empty or the API key may lack access.")
		os.Exit(0)
	}

	// ── PHASE 3: Deep analysis via Helius ────────────────────────────────────
	if cfg.HeliusAPIKey == "" {
		fmt.Println("\n[!] HELIUS_API_KEY not set — cannot calculate real win rates. Add it to .env")
		os.Exit(1)
	}

	fmt.Printf("\n[3/3] Analysing up to %d swaps per wallet via Helius to calculate real win rates...\n", cfg.HeliusTxLimit)

	var analyses []*models.WalletAnalysis
	passed, skipped := 0, 0

	for i, c := range filtered {
		select {
		case <-ctx.Done():
			fmt.Println("\n[!] Scan cancelled — saving partial results.")
			goto score
		default:
		}

		fmt.Printf("    [%d/%d] %s  pnl=$%.0f  trades=%d  periods=%d",
			i+1, len(filtered), c.Address[:8]+"...", c.PnL, c.TradeCount, c.PeriodCount)

		txs, err := helius.GetSwapTransactions(ctx, c.Address, cfg.HeliusTxLimit)
		if err != nil {
			fmt.Printf("  [helius err: %v]\n", err)
			skipped++
			time.Sleep(500 * time.Millisecond)
			continue
		}

		wa := analyzer.AnalyzeHistory(c.Address, txs, c)

		fmt.Printf("  swaps=%d  wr=%.1f%%  wins=%d  wdays=%d  wweeks=%d  top1=%.0f%%  idle=%dd",
			wa.SwapCount, wa.BirdeyeWinRate*100, wa.WinCount, wa.WinDays,
			wa.WinWeeks, wa.TopWinPct*100, wa.DaysSinceActive)

		// Apply filters using real Helius-computed data
		if !*noFilter {
			if c.PnL < cfg.MinPeriodPnLUSD {
				fmt.Printf("  [skip: period pnl $%.0f < $%.0f required]\n", c.PnL, cfg.MinPeriodPnLUSD)
				skipped++
				continue
			}
			if wa.TotalPnLSOL < cfg.MinHeliusPnLSOL {
				fmt.Printf("  [skip: helius pnl %.1f◎ < %.1f◎ required]\n", wa.TotalPnLSOL, cfg.MinHeliusPnLSOL)
				skipped++
				continue
			}
			if wa.BirdeyeWinRate < cfg.MinWinRate {
				fmt.Printf("  [skip: wr %.1f%% < %.0f%%]\n", wa.BirdeyeWinRate*100, cfg.MinWinRate*100)
				skipped++
				continue
			}
			if wa.WinCount < cfg.MinWinCount {
				fmt.Printf("  [skip: only %d wins < %d required]\n", wa.WinCount, cfg.MinWinCount)
				skipped++
				continue
			}
			if wa.WinDays < cfg.MinWinDays {
				fmt.Printf("  [skip: wins on only %d day(s) < %d required]\n", wa.WinDays, cfg.MinWinDays)
				skipped++
				continue
			}
			if cfg.MaxTopWinPct > 0 && wa.TopWinPct > cfg.MaxTopWinPct {
				fmt.Printf("  [skip: scraper — top win = %.0f%% of PnL]\n", wa.TopWinPct*100)
				skipped++
				continue
			}
			if wa.HistoryDays < cfg.MinHistoryDays {
				fmt.Printf("  [skip: hist %dd < %dd]\n", wa.HistoryDays, cfg.MinHistoryDays)
				skipped++
				continue
			}
			if cfg.MaxActiveAgoDays > 0 && wa.DaysSinceActive > cfg.MaxActiveAgoDays {
				fmt.Printf("  [skip: idle %dd > %dd]\n", wa.DaysSinceActive, cfg.MaxActiveAgoDays)
				skipped++
				continue
			}
		}

		wa.Score = scorer.Score(wa)
		analyses = append(analyses, wa)
		passed++
		fmt.Printf("  → score=%.2f ✓\n", wa.Score)

		time.Sleep(200 * time.Millisecond)
	}

score:
	fmt.Printf("\n[✓] %d passed  |  %d skipped\n", passed, skipped)

	if len(analyses) == 0 {
		fmt.Println("[!] No wallets survived all filters.")
		fmt.Println("    Try lowering MIN_WIN_RATE or MIN_HISTORY_DAYS in .env")
		if tg != nil {
			_ = tg.Send("🔍 *SOL Wallet Finder* — No qualifying wallets found today. Try again later.")
		}
		os.Exit(0)
	}

	ranked := scorer.Rank(analyses, cfg.TopN)
	output.PrintTable(ranked)
	output.PrintSummary(ranked)

	if err := output.SaveJSON(ranked, cfg.OutputFile); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Could not save JSON: %v\n", err)
	}

	// ── Send results to Telegram ─────────────────────────────────────────────
	if tg != nil {
		date := time.Now().Format("2006-01-02")
		msg := output.FormatTelegram(ranked, date)
		if err := tg.SendChunked(msg); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Telegram send failed: %v\n", err)
		} else {
			fmt.Println("[✓] Results sent to Telegram")
		}
	}

	if cfg.ExportForBot {
		// For the bot export: pick top 20 by PnL (most profitable wallets)
		top20ByPnL := scorer.RankByPnL(analyses, 20)
		fmt.Printf("\n[→] Top 20 by PnL being added to bot:\n")
		for _, w := range top20ByPnL {
			fmt.Printf("    #%-2d  %s  pnl=$%.0f  wr=%.1f%%\n",
				w.Rank, w.Address, w.TotalPnLUSD, w.WinRate)
		}
		if err := output.ExportForBot(top20ByPnL, cfg.BotWalletsFile); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Could not export to bot file: %v\n", err)
		}
	}
}
