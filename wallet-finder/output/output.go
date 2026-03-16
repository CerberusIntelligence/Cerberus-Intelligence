package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"wallet-finder/models"
)

// PrintTable prints a human-readable ranked table to stdout.
func PrintTable(ranked []models.RankedWallet) {
	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  %-4s  %-44s  %6s  %8s  %8s  %9s  %4s  %4s  %7s  %6s\n",
		"Rank", "Wallet Address", "Score", "AvgROI%", "AvgWin◎", "PnL USD", "Wins", "Days", "AvgHold", "Idle")
	fmt.Println("──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────")
	for _, w := range ranked {
		holdStr := "   n/a"
		if w.AvgHoldSeconds > 0 {
			mins := w.AvgHoldSeconds / 60
			if mins >= 60 {
				holdStr = fmt.Sprintf("%5.1fh", mins/60)
			} else {
				holdStr = fmt.Sprintf("%5.0fm", mins)
			}
		}
		fmt.Printf("  #%-3d  %-44s  %6.2f  %+7.0f%%  %7.2f◎  %+9.0f  %4d  %4d  %s  %4dd\n",
			w.Rank, w.Address, w.Score, w.AvgWinReturnPct, w.AvgWinSOL, w.TotalPnLUSD,
			w.WinCount, w.WinDays, holdStr, w.DaysSinceActive,
		)
	}
	fmt.Println("══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  AvgROI%%=avg return%% per winning trade  AvgWin=avg SOL profit per win  Days=distinct days with wins  AvgHold=avg hold time\n\n")
}

// SaveJSON writes the ranked wallet list to a JSON file.
func SaveJSON(ranked []models.RankedWallet, path string) error {
	data, err := json.MarshalIndent(ranked, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	fmt.Printf("[+] Results saved to %s\n", path)
	return nil
}

// ExportForBot writes a bot-compatible tracked_wallets.json where
// keys are wallet addresses and values are descriptive labels.
func ExportForBot(ranked []models.RankedWallet, path string) error {
	// Load existing file if present so we don't wipe existing wallets
	existing := make(map[string]string)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	added := 0
	for _, w := range ranked {
		if _, ok := existing[w.Address]; !ok {
			label := fmt.Sprintf("auto-found: score=%.1f wr=%.1f%% hist=%dd",
				w.Score, w.WinRate, w.HistoryDays)
			existing[w.Address] = label
			added++
		}
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write bot wallets file %s: %w", path, err)
	}

	fmt.Printf("[+] Added %d new wallets to bot file: %s\n", added, path)
	return nil
}

// FormatTelegram formats the ranked wallet list as a Telegram message.
func FormatTelegram(ranked []models.RankedWallet, date string) string {
	if len(ranked) == 0 {
		return "🔍 *SOL Wallet Finder* — " + date + "\n\nNo qualifying wallets found today. Try again later."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 *SOL Wallet Finder* — %s\n", date))
	sb.WriteString(fmt.Sprintf("Found *%d* qualifying wallets:\n\n", len(ranked)))

	for _, w := range ranked {
		sb.WriteString(fmt.Sprintf("*#%d* `%s`\n", w.Rank, w.Address))
		sb.WriteString(fmt.Sprintf(
			"  Score: %.1f | WR: %.1f%% | PnL: $%.0f\n",
			w.Score, w.WinRate, w.TotalPnLUSD,
		))
		sb.WriteString(fmt.Sprintf(
			"  Wins: %d | Weeks: %d | AvgWin: %.3f◎ | Top1%%: %.0f%% | Idle: %dd\n\n",
			w.WinCount, w.WinWeeks, w.AvgWinSOL, w.TopWinPct, w.DaysSinceActive,
		))
	}

	sb.WriteString("Reply with the addresses you want added to the bot.")
	return sb.String()
}

// PrintSummary prints a short summary after the table.
func PrintSummary(ranked []models.RankedWallet) {
	if len(ranked) == 0 {
		fmt.Println("[!] No wallets passed the filters. Try relaxing MIN_WIN_RATE or MIN_TRADES.")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[✓] Found %d qualifying wallets\n", len(ranked)))
	if len(ranked) > 0 {
		best := ranked[0]
		sb.WriteString(fmt.Sprintf("    Best: %s  score=%.2f  wr=%.1f%%  %d days history\n",
			best.Address, best.Score, best.WinRate, best.HistoryDays))
	}
	fmt.Print(sb.String())
}
