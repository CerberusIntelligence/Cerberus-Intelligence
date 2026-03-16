package discovery

import (
	"context"
	"fmt"
	"sort"
	"time"

	"wallet-finder/api"
)

// WalletHit tracks how many pumped tokens a wallet profited from.
type WalletHit struct {
	Address   string
	TokenWins int     // number of different pumped tokens this wallet bought early and sold at profit
	TotalPnL  float64 // total SOL profit across all tokens
}

// FindSmartMoney discovers wallets that consistently bought pumped tokens early and sold profitably.
// It works by:
//  1. Getting list of pumped tokens from DexScreener
//  2. For each token's liquidity pool, fetching early swap transactions via Helius
//  3. Finding wallets that appear as early profitable buyers across multiple tokens
func FindSmartMoney(ctx context.Context, helius *api.HeliusClient, tokens []api.PumpedToken, txLimit int) []string {
	// wallet address → number of tokens they profited from
	hits := make(map[string]*WalletHit)

	for i, tok := range tokens {
		select {
		case <-ctx.Done():
			break
		default:
		}

		fmt.Printf("  [token %d/%d] %s  +%.0f%%  vol=$%.0fK  liq=$%.0fK\n",
			i+1, len(tokens), tok.Symbol, tok.PriceChange24h,
			tok.VolumeUSD/1000, tok.LiquidityUSD/1000)

		// Get transactions for this liquidity pool
		txs, err := helius.GetPoolTransactions(ctx, tok.PairAddress, txLimit)
		if err != nil {
			fmt.Printf("    [helius err: %v]\n", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if len(txs) < 5 {
			fmt.Printf("    [skip: only %d txs]\n", len(txs))
			continue
		}

		// Sort by time ascending
		sort.Slice(txs, func(a, b int) bool {
			return txs[a].Timestamp < txs[b].Timestamp
		})

		// Find early buyers: wallets in the first 30% of transactions that also sold later
		cutoff := len(txs) * 30 / 100
		if cutoff < 3 {
			cutoff = 3
		}

		earlyBuyers := make(map[string]float64) // wallet → SOL spent buying
		for _, tx := range txs[:cutoff] {
			wallet := tx.FeePayer
			if wallet == "" {
				continue
			}
			// Detect a buy: wallet lost SOL (negative native balance change)
			for _, acct := range tx.AccountData {
				if acct.Account == wallet && acct.NativeBalChange < 0 {
					earlyBuyers[wallet] += float64(-acct.NativeBalChange) / 1_000_000_000.0
				}
			}
		}

		// Check which early buyers also appear as sellers later (took profit)
		laterSellers := make(map[string]float64) // wallet → SOL received selling
		for _, tx := range txs[cutoff:] {
			wallet := tx.FeePayer
			if _, wasBuyer := earlyBuyers[wallet]; !wasBuyer {
				continue
			}
			for _, acct := range tx.AccountData {
				if acct.Account == wallet && acct.NativeBalChange > 0 {
					laterSellers[wallet] += float64(acct.NativeBalChange) / 1_000_000_000.0
				}
			}
		}

		// Record wallets that bought early and sold at profit
		winners := 0
		for wallet, solSpent := range earlyBuyers {
			solReceived, sold := laterSellers[wallet]
			if !sold {
				continue
			}
			profit := solReceived - solSpent
			if profit <= 0 {
				continue // didn't profit
			}
			if hits[wallet] == nil {
				hits[wallet] = &WalletHit{Address: wallet}
			}
			hits[wallet].TokenWins++
			hits[wallet].TotalPnL += profit
			winners++
		}

		fmt.Printf("    early buyers=%d  profitable exits=%d\n", len(earlyBuyers), winners)
		time.Sleep(200 * time.Millisecond)
	}

	// Sort by number of token wins (wallets winning across most tokens = most consistent)
	var ranked []*WalletHit
	for _, h := range hits {
		if h.TokenWins >= 2 { // must have won on at least 2 different tokens
			ranked = append(ranked, h)
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].TokenWins != ranked[j].TokenWins {
			return ranked[i].TokenWins > ranked[j].TokenWins
		}
		return ranked[i].TotalPnL > ranked[j].TotalPnL
	})

	fmt.Printf("\n  Smart money candidates: %d wallets won on 2+ tokens\n", len(ranked))

	var addrs []string
	for _, h := range ranked {
		fmt.Printf("    %s  token_wins=%d  est_pnl=%.2f◎\n", h.Address, h.TokenWins, h.TotalPnL)
		addrs = append(addrs, h.Address)
	}
	return addrs
}
