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
	log.Infof("Channels configured: %d", len(cfg.MonitoredChannels))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Core engine
	eng := engine.New(cfg)

	// Signal monitor (extracts contract addresses from messages)
	monitor := telegram.NewMonitor(eng.SignalCh())

	// Telegram control bot (alerts + commands)
	bot := telegram.NewBot(cfg, eng)
	go bot.Start(ctx)

	// MTProto listener (reads alpha channels) — skipped in copy-only mode
	if cfg.CopyOnlyMode {
		log.Info("Copy-only mode — channel monitoring disabled, wallet tracker active")
	} else {
		mtproto := telegram.NewMTProtoClient(cfg, log.StandardLogger(), monitor)
		if err := mtproto.Connect(ctx); err != nil {
			log.WithError(err).Fatal("MTProto connection failed")
		}
	}

	// Start engine
	go eng.Start(ctx)

	log.Info("Bot running. Press Ctrl+C to stop.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info("Shutting down...")
	cancel()
}
