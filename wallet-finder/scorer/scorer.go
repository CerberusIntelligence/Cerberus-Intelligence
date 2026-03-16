package scorer

import (
	"math"
	"sort"

	"wallet-finder/models"
)

const (
	weightWinRate     = 0.20 // raw win rate
	weightConsistency = 0.25 // stable win rate across months
	weightSpread      = 0.35 // wins spread across many weeks/days — most important
	weightHistory     = 0.15 // depth of trading history
	weightRecency     = 0.05 // recently active
)

// Score computes a composite 0–100 score.
// Prioritises consistent, long-history traders over one-hit wonders.
// Rewards both scalpers (many small wins per week) and swing traders
// (high % gain per trade) as long as wins are spread across multiple weeks.
func Score(wa *models.WalletAnalysis) float64 {
	if wa == nil {
		return 0
	}

	winRateNorm := normaliseWinRate(wa.BirdeyeWinRate)

	// Win spread: weeks are the primary signal (any active trader can win in 1 day).
	// Full score at 4+ distinct weeks with wins — proves it's not a fluke.
	spreadScore := 0.0
	if wa.WinCount > 0 {
		// Week spread: full score at 3+ weeks — proves multi-week consistency
		weekScore := math.Min(1.0, float64(wa.WinWeeks)/3.0)
		// Day spread: full score at 14+ winning days — proves sustained trading
		dayScore := math.Min(1.0, float64(wa.WinDays)/14.0)
		// Win count: full score at 30+ wins — volume of consistent winners
		countScore := math.Min(1.0, float64(wa.WinCount)/30.0)
		spreadScore = weekScore*0.5 + dayScore*0.35 + countScore*0.15
	}

	composite := winRateNorm*weightWinRate +
		wa.ConsistencyScore*weightConsistency +
		spreadScore*weightSpread +
		wa.HistoryScore*weightHistory +
		wa.RecencyScore*weightRecency

	// Scraper penalty: if the single biggest win is >50% of total PnL,
	// this is likely a one-hit wonder / lucky sniper — penalise hard.
	if wa.TopWinPct > 0.50 {
		penalty := (wa.TopWinPct - 0.50) / 0.50 // 0 at 50%, 1.0 at 100%
		composite *= (1.0 - penalty*0.60)         // up to -60% penalty
	}

	// Multi-period bonus
	periodBonus := map[int]float64{1: 0, 2: 0.03, 3: 0.06, 4: 0.10}
	bonus := periodBonus[wa.PeriodCount]
	composite = math.Min(1.0, composite*(1+bonus))

	return math.Round(composite*10000) / 100
}

func normaliseWinRate(wr float64) float64 {
	if wr <= 0.5 {
		return 0
	}
	return math.Sqrt((wr - 0.5) / 0.5)
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
			PeriodCount:      wa.PeriodCount,
			ConsistencyScore: math.Round(wa.ConsistencyScore*100) / 100,
			HistoryDays:      wa.HistoryDays,
			DaysSinceActive:  wa.DaysSinceActive,
			MonthlyWinRates:  wa.MonthlyStats,
		})
	}
	return ranked
}
