package telegram

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"solana-trading-bot/types"

	log "github.com/sirupsen/logrus"
)

var (
	// URL patterns — most reliable, extract from DEX/explorer links
	pumpFunRe = regexp.MustCompile(`pump\.fun/coin/([1-9A-HJ-NP-Za-km-z]{32,44})`)
	dexRe     = regexp.MustCompile(`dexscreener\.com/solana/([1-9A-HJ-NP-Za-km-z]{32,44})`)
	birdeyeRe = regexp.MustCompile(`birdeye\.so/token/([1-9A-HJ-NP-Za-km-z]{32,44})`)
	solscanRe = regexp.MustCompile(`solscan\.io/token/([1-9A-HJ-NP-Za-km-z]{32,44})`)
	// Raw base58 addresses (fallback)
	rawAddrRe = regexp.MustCompile(`\b([1-9A-HJ-NP-Za-km-z]{32,44})\b`)
	// Achievement patterns — "TOKEN has reached 3x" or "$TOKEN 3x from call"
	achieveRe1 = regexp.MustCompile(`(?i)\$?([A-Za-z0-9_]+)\s+has\s+reached\s+(\d+(?:\.\d+)?)x`)
	achieveRe2 = regexp.MustCompile(`(?i)\$?([A-Za-z0-9_]+)\s+(\d+(?:\.\d+)?)x\s+from\s+call`)
	// Retrospective gem updates — "MEMECOIN GEM UPDATES $TOKEN ... Nx From Private Gem Alert"
	// These announce gains that ALREADY happened — treat as achievement only, never buy
	gemUpdateRe = regexp.MustCompile(`(?i)MEMECOIN\s+GEM\s+UPDATES\s+\$?([A-Za-z0-9_]+)[\s\S]{0,120}?(\d+(?:\.\d+)?)x\s+From\s+Private`)
)

// Well-known program/token addresses to ignore
var ignoreList = map[string]bool{
	"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA": true,
	"So11111111111111111111111111111111111111112":   true,
	"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v": true,
	"Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB": true,
	"11111111111111111111111111111111":              true,
	"ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJe1bS": true,
}

// Monitor extracts Solana contract addresses from messages and sends signals.
type Monitor struct {
	signalCh chan<- types.Signal
}

// NewMonitor creates a monitor that forwards signals to the given channel.
func NewMonitor(signalCh chan<- types.Signal) *Monitor {
	return &Monitor{signalCh: signalCh}
}

// ProcessMessage parses a channel message and emits signals.
func (m *Monitor) ProcessMessage(source string, msgID int, text string) {
	preview := text
	if len(preview) > 80 {
		preview = preview[:80] + "..."
	}
	log.WithFields(log.Fields{
		"source": source,
		"msg":    preview,
	}).Info("Channel message received")

	// Check for retrospective gem updates FIRST — never buy these, only extend holds
	if sym, mult := extractGemUpdate(text); sym != "" {
		log.WithFields(log.Fields{
			"source": source,
			"symbol": sym,
			"mult":   fmt.Sprintf("%.1fx", mult),
		}).Info("Gem update (retrospective) — achievement signal only, no re-entry")
		select {
		case m.signalCh <- types.Signal{
			IsAchievement:   true,
			IsRetrospective: true,
			Symbol:          sym,
			Multiplier:      mult,
			Source:          source,
			Message:         text,
			Timestamp:       time.Now(),
		}:
		default:
		}
		return
	}

	// Check for achievement messages — "TOKEN has reached Nx"
	if sym, mult := extractAchievement(text); sym != "" {
		log.WithFields(log.Fields{
			"source": source,
			"symbol": sym,
			"mult":   fmt.Sprintf("%.1fx", mult),
		}).Info("Achievement detected — extending hold")
		select {
		case m.signalCh <- types.Signal{
			IsAchievement: true,
			Symbol:        sym,
			Multiplier:    mult,
			Source:        source,
			Message:       text,
			Timestamp:     time.Now(),
		}:
		default:
		}
		return // achievement messages rarely have addresses, skip address scan
	}

	// Regular contract address signals
	addresses := extractAddresses(text)
	if len(addresses) == 0 {
		log.WithField("source", source).Debug("No contract address found in message")
		return
	}

	for _, addr := range addresses {
		log.WithFields(log.Fields{
			"source":  source,
			"address": addr[:8] + "...",
		}).Info("Signal detected")

		select {
		case m.signalCh <- types.Signal{
			Address:   addr,
			Source:    source,
			Message:   text,
			Timestamp: time.Now(),
		}:
		default:
			log.Warn("Signal channel full, dropping signal")
		}
	}
}

func extractGemUpdate(text string) (symbol string, multiplier float64) {
	if m := gemUpdateRe.FindStringSubmatch(text); m != nil {
		mult, _ := strconv.ParseFloat(m[2], 64)
		return strings.TrimPrefix(m[1], "$"), mult
	}
	return "", 0
}

func extractAchievement(text string) (symbol string, multiplier float64) {
	if m := achieveRe1.FindStringSubmatch(text); m != nil {
		mult, _ := strconv.ParseFloat(m[2], 64)
		return strings.TrimPrefix(m[1], "$"), mult
	}
	if m := achieveRe2.FindStringSubmatch(text); m != nil {
		mult, _ := strconv.ParseFloat(m[2], 64)
		return strings.TrimPrefix(m[1], "$"), mult
	}
	return "", 0
}

func extractAddresses(text string) []string {
	seen := make(map[string]bool)
	var result []string

	add := func(addr string) {
		if !seen[addr] && !ignoreList[addr] {
			seen[addr] = true
			result = append(result, addr)
		}
	}

	// Priority 1: URL patterns (most reliable)
	for _, re := range []*regexp.Regexp{pumpFunRe, dexRe, birdeyeRe, solscanRe} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			add(m[1])
		}
	}

	// Priority 2: raw addresses (if no URL matches)
	if len(result) == 0 {
		for _, addr := range rawAddrRe.FindAllString(text, -1) {
			if looksLikeSolanaAddress(addr) {
				add(addr)
			}
		}
	}

	return result
}

func looksLikeSolanaAddress(s string) bool {
	if len(s) < 32 {
		return false
	}
	// pump.fun tokens always end in "pump"
	if strings.HasSuffix(s, "pump") {
		return true
	}
	// Standard addresses are typically 43-44 chars
	return len(s) >= 43
}
