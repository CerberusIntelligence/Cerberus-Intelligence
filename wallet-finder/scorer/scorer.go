package scorer

import (
	"math"
	"sort"

	"wallet-finder/models"
)

const (
	weightReturn      = 0.40 // avg ROI % per trade — primary signal
	weightSpread      = 0.30 // wins spread across days/weeks — consistency
	weightHold        = 0.15 // avg hold time — filters bots/snipers
	weightRecency     = 0.10 // recently active
	weightHistory     = 0.05 // depth of trading history
)

// Score computes a composite 0–100 score.
// Prioritises high return % per trade and consistency over raw win rate.
func Score(wa *models.WalletAnalysis) float64 {
	if wa == nil {
		return 0
	}

	// Return score: avg win return %. Full score at 200%+ avg return per win.
	// A 200% return means tripling money on average — that's exceptional.
	returnScore := 0.0
	if wa.AvgWinReturnPct > 0 {
		returnScore = math.Min(1.0, wa.AvgWinReturnPct/200.0)
	}

	// Spread: wins across multiple days and weeks proves it's not a fluke
	spreadScore := 0.0
	if wa.WinCount > 0 {
		weekScore := math.Min(1.0, float64(wa.WinWeeks)/3.0)
		dayScore := math.Min(1.0, float64(wa.WinDays)/7.0)
		countScore := math.Min(1.0, float64(wa.WinCount)/20.0)
		spreadScore = weekScore*0.5 + dayScore*0.35 + countScore*0.15
	}

	// Hold score: full score at 60+ min avg hold. Bots hold seconds, humans hold minutes/hours.
	holdScore := 0.0
	if wa.AvgHoldSeconds > 0 {
		holdScore = math.Min(1.0, wa.AvgHoldSeconds/3600.0) // full at 1h
	}

	composite := returnScore*weightReturn +
		spreadScore*weightSpread +
		holdScore*weightHold +
		wa.RecencyScore*weightRecency +
		wa.HistoryScore*weightHistory

	// Scraper penalty: one trade dominating total PnL = lucky fluke not skill
	if wa.TopWinPct > 0.50 {
		penalty := (wa.TopWinPct - 0.50) / 0.50
		composite *= (1.0 - penalty*0.50)
	}

	return math.Round(composite*10000) / 100
}

func Rank(analyses []*models.WalletAnalysis, topN int) []models.RankedWallet {
	return rankBy(analyses, topN, func(a, b *models.WalletAnalysis) bool {
		return a.Score > b.Score
	})
}

func RankByPnL(analyses []*models.WalletAnalysis, topN int) []models.RankedWallet {
	return rankBy(analyses, topN, func(a, b *models.WalletAnalysis) bool {
		return a.BirdeyePnL > b.BirdeyePnL
	})
}

func rankBy(analyses []*models.WalletAnalysis, topN int, less func(a, b *models.WalletAnalysis) bool) []models.RankedWallet {
	cp := make([]*models.WalletAnalysis, len(analyses))
	copy(cp, analyses)
	sort.Slice(cp, func(i, j int) bool { return less(cp[i], cp[j]) })

	if topN > len(cp) {
		topN = len(cp)
	}

	ranked := make([]models.RankedWallet, 0, topN)
	for i, wa := range cp[:topN] {
		ranked = append(ranked, models.RankedWallet{
			Rank:             i + 1,
			Address:          wa.Address,
			Score:            wa.Score,
			WinRate:          math.Round(wa.BirdeyeWinRate*10000) / 100,
			TotalPnLUSD:      math.Round(wa.BirdeyePnL*100) / 100,
			TotalTrades:      wa.BirdeyeTrades,
			WinCount:         wa.WinCount,
			WinDays:          wa.WinDays,
			WinWeeks:         wa.WinWeeks,
			AvgWinSOL:        math.Round(wa.AvgWinSOL*1000) / 1000,
			AvgLossSOL:       math.Round(wa.AvgLossSOL*1000) / 1000,
			TopWinPct:        math.Round(wa.TopWinPct*1000) / 10,
			AvgReturnPct:     math.Round(wa.AvgReturnPct*10) / 10,
			AvgWinReturnPct:  math.Round(wa.AvgWinReturnPct*10) / 10,
			PeriodCount:      wa.PeriodCount,
			ConsistencyScore: math.Round(wa.ConsistencyScore*100) / 100,
			HistoryDays:      wa.HistoryDays,
			DaysSinceActive:  wa.DaysSinceActive,
			AvgHoldSeconds:   math.Round(wa.AvgHoldSeconds),
			MonthlyWinRates:  wa.MonthlyStats,
		})
	}
	return ranked
}
