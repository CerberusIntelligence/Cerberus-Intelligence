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
	"wallet-finder/config"
	"wallet-finder/models"
	"wallet-finder/output"
	"wallet-finder/scorer"
)

func main() {
	minWinRate := flag.Float64("min-wr", 0, "Override MIN_WIN_RATE (e.g. 0.60)")
	minTrades := flag.Int("min-trades", 0, "Override MIN_TRADES (e.g. 50)")
	minHistory := flag.Int("min-history", 0, "Override MIN_HISTORY_DAYS (e.g. 30)")
	topN := flag.Int("top", 0, "Override TOP_N (e.g. 100)")
	exportBot := flag.Bool("export-bot", false, "Export top wallets to bot's tracked_wallets.json")
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
		os.Exit(0)
	}

	ranked := scorer.Rank(analyses, cfg.TopN)
	output.PrintTable(ranked)
	output.PrintSummary(ranked)

	if err := output.SaveJSON(ranked, cfg.OutputFile); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Could not save JSON: %v\n", err)
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
