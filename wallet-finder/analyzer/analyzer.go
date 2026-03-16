package analyzer

import (
	"fmt"
	"math"
	"sort"
	"time"

	"wallet-finder/api"
	"wallet-finder/models"
)

const (
	lamportsPerSOL = 1_000_000_000.0
	wsolMint       = "So11111111111111111111111111111111111111112"
	minSolThresh   = 0.001
)

type tokenFlow struct {
	solIn    float64
	solOut   float64
	sold     bool
	day      string // YYYY-MM-DD of last activity
	buyTime  int64  // unix timestamp of first buy
	sellTime int64  // unix timestamp of last sell
}

func AnalyzeHistory(address string, txs []api.HeliusTx, candidate api.BirdeyeCandidate) *models.WalletAnalysis {
	wa := &models.WalletAnalysis{
		Address:          address,
		BirdeyePnL:       candidate.PnL,
		BirdeyeTrades:    candidate.TradeCount,
		PeriodCount:      candidate.PeriodCount,
		SwapCount:        len(txs),
		ConsistencyScore: 0.5,
	}

	if len(txs) == 0 {
		return wa
	}

	sort.Slice(txs, func(i, j int) bool {
		return txs[i].Timestamp < txs[j].Timestamp
	})

	firstAt := time.Unix(txs[0].Timestamp, 0)
	lastAt := time.Unix(txs[len(txs)-1].Timestamp, 0)
	wa.HistoryDays = int(lastAt.Sub(firstAt).Hours() / 24)
	wa.DaysSinceActive = int(time.Since(lastAt).Hours() / 24)

	flows := make(map[string]*tokenFlow)

	for _, tx := range txs {
		solDelta := walletSolDelta(tx, address)
		day := time.Unix(tx.Timestamp, 0).Format("2006-01-02")

		if solDelta < -minSolThresh {
			token := receivedToken(tx, address)
			if token == "" || token == wsolMint {
				continue
			}
			if flows[token] == nil {
				flows[token] = &tokenFlow{}
			}
			flows[token].solIn += math.Abs(solDelta)
			flows[token].day = day
			if flows[token].buyTime == 0 {
				flows[token].buyTime = tx.Timestamp
			}
		} else if solDelta > minSolThresh {
			token := sentToken(tx, address)
			if token == "" || token == wsolMint {
				continue
			}
			if flows[token] == nil {
				flows[token] = &tokenFlow{}
			}
			flows[token].solOut += solDelta
			flows[token].sold = true
			flows[token].day = day
			flows[token].sellTime = tx.Timestamp
		}
	}

	// ── Tally wins/losses ───────────────────────────────────────────────────
	type monthBucket struct{ wins, losses int }
	months := make(map[string]*monthBucket)
	winDaySet := make(map[string]bool)
	winWeekSet := make(map[string]bool)

	var winAmounts []float64
	var lossAmounts []float64
	var holdTimes []float64
	var allReturns []float64
	var winReturns []float64
	totalPnL := 0.0
	biggestWin := 0.0

	for _, f := range flows {
		if !f.sold {
			continue
		}
		pnl := f.solOut - f.solIn

		// Compute hold duration in seconds
		holdSecs := int64(0)
		if f.buyTime > 0 && f.sellTime > 0 {
			holdSecs = f.sellTime - f.buyTime
		}

		// Discard wins held less than 60 seconds — bots and snipers flip in
		// seconds; real traders hold. Losses are allowed to be short (stop-loss).
		if pnl >= 0 && holdSecs > 0 && holdSecs < 60 {
			continue
		}

		totalPnL += pnl

		// ROI % for this position: (profit / cost) * 100
		returnPct := 0.0
		if f.solIn > 0 {
			returnPct = (pnl / f.solIn) * 100
		}
		allReturns = append(allReturns, returnPct)

		month := f.day[:7] // YYYY-MM
		if months[month] == nil {
			months[month] = &monthBucket{}
		}

		if pnl >= 0 {
			wa.WinCount++
			winAmounts = append(winAmounts, pnl)
			winReturns = append(winReturns, returnPct)
			if holdSecs > 0 {
				holdTimes = append(holdTimes, float64(holdSecs))
			}
			winDaySet[f.day] = true
			// ISO week key: YYYY-Www
			t := time.Unix(0, 0)
			if parsed, err := time.Parse("2006-01-02", f.day); err == nil {
				t = parsed
			}
			yr, wk := t.ISOWeek()
			winWeekSet[fmt.Sprintf("%d-W%02d", yr, wk)] = true
			months[month].wins++
			if pnl > biggestWin {
				biggestWin = pnl
			}
		} else {
			wa.LossCount++
			lossAmounts = append(lossAmounts, math.Abs(pnl))
			months[month].losses++
		}
	}

	if len(holdTimes) > 0 {
		sum := 0.0
		for _, h := range holdTimes {
			sum += h
		}
		wa.AvgHoldSeconds = sum / float64(len(holdTimes))
	}
	if len(allReturns) > 0 {
		sum := 0.0
		for _, r := range allReturns {
			sum += r
		}
		wa.AvgReturnPct = sum / float64(len(allReturns))
	}
	if len(winReturns) > 0 {
		sum := 0.0
		for _, r := range winReturns {
			sum += r
		}
		wa.AvgWinReturnPct = sum / float64(len(winReturns))
	}

	total := wa.WinCount + wa.LossCount
	if total > 0 {
		wa.BirdeyeWinRate = float64(wa.WinCount) / float64(total)
	}
	wa.TotalPnLSOL = totalPnL
	wa.WinDays = len(winDaySet)
	wa.WinWeeks = len(winWeekSet)

	if len(winAmounts) > 0 {
		sum := 0.0
		for _, v := range winAmounts {
			sum += v
		}
		wa.AvgWinSOL = sum / float64(len(winAmounts))
	}
	if len(lossAmounts) > 0 {
		sum := 0.0
		for _, v := range lossAmounts {
			sum += v
		}
		wa.AvgLossSOL = sum / float64(len(lossAmounts))
	}

	// TopWinPct: what % of total profit came from the single biggest win.
	// A legit consistent trader: low value (e.g. 20%).
	// A scraper/one-hit wonder: high value (e.g. 95%).
	if totalPnL > 0 && biggestWin > 0 {
		wa.TopWinPct = biggestWin / totalPnL
	}

	// ── Monthly stats & consistency ─────────────────────────────────────────
	type keyVal struct {
		k string
		v *monthBucket
	}
	var sorted []keyVal
	for k, v := range months {
		sorted = append(sorted, keyVal{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].k < sorted[j].k })

	var winRates []float64
	for _, kv := range sorted {
		t := kv.v.wins + kv.v.losses
		wr := 0.0
		if t > 0 {
			wr = float64(kv.v.wins) / float64(t)
		}
		wa.MonthlyStats = append(wa.MonthlyStats, models.MonthlyStats{
			YearMonth: kv.k,
			Wins:      kv.v.wins,
			Losses:    kv.v.losses,
			WinRate:   wr,
		})
		if t >= 3 {
			winRates = append(winRates, wr)
		}
	}

	wa.ConsistencyScore = consistencyScore(winRates)
	// HistoryScore: cap at 30 days (1 month) = 1.0.
	// We're now requiring 14+ days minimum so this scale makes sense.
	wa.HistoryScore = math.Min(1.0, float64(wa.HistoryDays)/30.0)
	wa.RecencyScore = math.Max(0.0, 1.0-float64(wa.DaysSinceActive)/30.0)

	return wa
}

func walletSolDelta(tx api.HeliusTx, address string) float64 {
	for _, acct := range tx.AccountData {
		if acct.Account == address {
			return float64(acct.NativeBalChange) / lamportsPerSOL
		}
	}
	return 0
}

func receivedToken(tx api.HeliusTx, address string) string {
	for _, tt := range tx.TokenTransfers {
		if tt.ToUserAccount == address && tt.Mint != wsolMint && tt.TokenAmount > 0 {
			return tt.Mint
		}
	}
	return ""
}

func sentToken(tx api.HeliusTx, address string) string {
	for _, tt := range tx.TokenTransfers {
		if tt.FromUserAccount == address && tt.Mint != wsolMint && tt.TokenAmount > 0 {
			return tt.Mint
		}
	}
	return ""
}

func consistencyScore(rates []float64) float64 {
	if len(rates) < 2 {
		return 0.5
	}
	mean := 0.0
	for _, r := range rates {
		mean += r
	}
	mean /= float64(len(rates))
	variance := 0.0
	for _, r := range rates {
		d := r - mean
		variance += d * d
	}
	variance /= float64(len(rates))
	return math.Max(0.0, 1.0-math.Sqrt(variance)/0.5)
}

// monthKey returns "YYYY-MM" for a unix timestamp.
func monthKey(ts int64) string {
	t := time.Unix(ts, 0)
	return fmt.Sprintf("%d-%02d", t.Year(), int(t.Month()))
}
