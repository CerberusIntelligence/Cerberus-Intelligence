package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"solana-trading-bot/config"
	"solana-trading-bot/engine"
	"solana-trading-bot/telegram"

	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.SetLevel(log.InfoLevel)

	cfg := config.Load()

	mode := "PAPER TRADING"
	if !cfg.PaperTrading {
		mode = "⚠️  LIVE TRADING"
	}

	fmt.Println("==========================================")
	fmt.Println("       SOLANA ALPHA BOT v2.0              ")
	fmt.Println("==========================================")
	log.Infof("Mode: %s", mode)
	log.Infof("Trade amount: %.4f SOL", cfg.TradeAmountSOL)
	log.Infof("Max positions: %d", cfg.MaxPositions)
	log.Infof("Stop loss: %.0f%% | Trailing: %.0f%%", cfg.StopLossPct*100, cfg.TrailingStopPct*100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Core engine
	eng := engine.New(cfg)

	// Telegram control bot (alerts + commands)
	bot := telegram.NewBot(cfg, eng)
	go bot.Start(ctx)

	// Start engine
	go eng.Start(ctx)

	log.Info("Bot running. Press Ctrl+C to stop.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info("Shutting down...")
	cancel()
}
