# Solana Trading Bot

A high-speed Go-based trading bot for Solana memecoins with Telegram signal monitoring, rug pull protection, wallet tracking, and social sentiment analysis.

## Features

- **Telegram Channel Monitoring** - Monitors specified channels for token mentions
- **Rug Pull Protection** - Multi-layer safety checks:
  - Liquidity lock verification
  - Mint/freeze authority checks
  - Holder distribution analysis
  - Honeypot detection via RugCheck API
- **Wallet Copy Trading** - Track and copy successful traders
- **Twitter Sentiment** - Monitor social signals for hold/sell decisions
- **Sniper Mode** - Fast execution with priority fees via Jupiter
- **Smart Exit Strategy** - Take profit levels, trailing stops, timeout exits
- **Risk Management** - 2% risk per trade, position sizing

## Quick Start

### 1. Prerequisites

- Go 1.21 or higher
- A Solana wallet with SOL
- Telegram Bot Token
- (Optional) Twitter API access

### 2. Installation

```bash
# Clone or navigate to the bot directory
cd solana-trading-bot

# Install dependencies
go mod tidy

# Copy and configure environment
cp .env.example .env
```

### 3. Configuration

Edit `.env` with your settings:

```env
# Required
SOLANA_PRIVATE_KEY=your_base58_private_key
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
TELEGRAM_CHAT_ID=your_chat_id

# Channels to monitor
MONITORED_CHANNELS=channel1,channel2

# Wallets to copy
TRACKED_WALLETS=wallet1,wallet2
```

### 4. Run

```bash
go run main.go
```

## Configuration Guide

### Trading Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `RISK_PER_TRADE` | 0.02 | Risk 2% of portfolio per trade |
| `MAX_POSITION_SOL` | 0.1 | Maximum SOL per trade |
| `MIN_LIQUIDITY_USD` | 10000 | Minimum pool liquidity |
| `SLIPPAGE_BPS` | 500 | 5% slippage tolerance |
| `PRIORITY_FEE_LAMPORTS` | 100000 | Priority fee for speed |

### Safety Thresholds

| Setting | Default | Description |
|---------|---------|-------------|
| `MIN_LP_LOCKED_PERCENT` | 80 | Minimum LP locked |
| `MAX_TOP_HOLDER_PERCENT` | 10 | Max single holder |
| `MAX_DEV_WALLET_PERCENT` | 5 | Max dev holdings |
| `MIN_HOLDER_COUNT` | 100 | Minimum holders |

### Exit Strategy

| Setting | Default | Description |
|---------|---------|-------------|
| `TAKE_PROFIT_LEVELS` | 2.0,5.0,10.0 | Sell at 2x, 5x, 10x |
| `TAKE_PROFIT_PERCENTS` | 0.3,0.3,0.4 | Sell 30%, 30%, 40% |
| `STOP_LOSS_PERCENT` | 0.5 | Stop loss at -50% |
| `TRAILING_STOP_PERCENT` | 0.2 | 20% trailing stop |

## Telegram Commands

| Command | Description |
|---------|-------------|
| `/start` | Resume trading |
| `/stop` | Pause trading |
| `/status` | Show bot statistics |
| `/positions` | Show open positions |
| `/balance` | Check SOL balance |
| `/add_wallet <addr>` | Add wallet to track |
| `/remove_wallet <addr>` | Remove tracked wallet |
| `/sell <token>` | Manually sell position |
| `/sell_all` | Close all positions |

## Safety Checks

The bot performs these checks before buying:

1. **Contract Analysis**
   - Mint authority revoked
   - Freeze authority disabled
   - No dangerous functions

2. **Liquidity Verification**
   - Minimum liquidity threshold
   - LP tokens locked/burned
   - Lock duration check

3. **Holder Distribution**
   - Top holder percentage
   - Dev wallet detection
   - Minimum holder count

4. **Honeypot Detection**
   - RugCheck API integration
   - Sell simulation
   - Tax analysis

## Architecture

```
solana-trading-bot/
├── main.go              # Entry point
├── config/              # Configuration loading
├── telegram/            # Telegram monitor & bot
├── solana/              # Blockchain client & Jupiter
├── safety/              # Rug pull checks
├── tracker/             # Wallet tracking
├── social/              # Twitter monitoring
├── engine/              # Trading logic
└── types/               # Shared types
```

## RPC Recommendations

For best performance, use a paid RPC:

- **Helius** - Recommended for Solana trading
- **QuickNode** - Good performance
- **Triton** - Specialized for trading

Free RPC will work but may be rate-limited.

## Risk Warning

**This bot trades real money. Use at your own risk.**

- Start with small amounts
- Test thoroughly before scaling
- Never risk more than you can afford to lose
- Memecoins are extremely volatile
- Past performance doesn't guarantee future results

## Troubleshooting

### Bot not receiving signals
- Ensure Telegram API credentials are correct
- Check that channels are accessible to your account
- Verify channel usernames (without @)

### Trades failing
- Check SOL balance for fees
- Increase slippage if needed
- Verify RPC endpoint is working
- Check token has sufficient liquidity

### Safety checks too strict
- Adjust thresholds in `.env`
- Some legitimate tokens may fail checks
- Manual override available via commands

## License

MIT License - Use at your own risk.
