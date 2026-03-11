package solana

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"solana-trading-bot/config"
	"solana-trading-bot/types"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/sirupsen/logrus"
)

const (
	WSOL = "So11111111111111111111111111111111111111112"
)

// sharedTransport is a tuned HTTP transport reused across all clients in this package.
// Connection pooling eliminates TCP handshake overhead on repeated API calls.
var sharedTransport = &http.Transport{
	MaxIdleConns:        200,
	MaxIdleConnsPerHost: 20,
	IdleConnTimeout:     90 * time.Second,
	DisableCompression:  true, // skip gzip on hot paths — saves CPU
}

// Client handles Solana blockchain interactions
type Client struct {
	cfg        *config.Config
	log        *logrus.Logger
	rpc        *rpc.Client   // current primary (fastest observed) endpoint
	rpcAll     []*rpc.Client // all endpoints — used for tx blasting
	wallet     solana.PrivateKey
	httpClient *http.Client // 8s timeout — Jupiter quote/swap, token metadata
	fastClient *http.Client // 3s timeout — price checks, quick metadata
	mu         sync.RWMutex
}

// NewClient creates a new Solana client
func NewClient(cfg *config.Config, log *logrus.Logger) (*Client, error) {
	urls := []string{cfg.SolanaRPCURL}
	if len(cfg.AdditionalRPCURLs) > 0 {
		urls = append(urls, cfg.AdditionalRPCURLs...)
	}

	clients := make([]*rpc.Client, 0, len(urls))
	for _, u := range urls {
		if u == "" {
			continue
		}
		clients = append(clients, rpc.New(u))
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("no RPC endpoints configured")
	}

	wallet, err := solana.PrivateKeyFromBase58(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	log.WithField("address", wallet.PublicKey().String()).Info("Wallet loaded")

	return &Client{
		cfg:    cfg,
		log:    log,
		rpc:    clients[0],
		rpcAll: clients,
		wallet: wallet,
		httpClient: &http.Client{
			Timeout:   8 * time.Second,
			Transport: sharedTransport,
		},
		fastClient: &http.Client{
			Timeout:   3 * time.Second,
			Transport: sharedTransport,
		},
	}, nil
}

// getRPC returns the current primary RPC client
func (c *Client) getRPC() *rpc.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rpc
}

// withRPCFailover runs fn against the primary RPC, falling back to others on error.
// For write operations (sending transactions), use blastSendTransaction instead.
func (c *Client) withRPCFailover(fn func(r *rpc.Client) error) error {
	c.mu.RLock()
	current := c.rpc
	all := c.rpcAll
	c.mu.RUnlock()

	if err := fn(current); err == nil {
		return nil
	}

	for _, client := range all {
		if client == current {
			continue
		}
		if err := fn(client); err == nil {
			c.mu.Lock()
			c.rpc = client
			c.mu.Unlock()
			return nil
		}
	}

	return fn(current)
}

// blastSendTransaction sends a transaction to ALL configured RPC endpoints simultaneously
// and returns as soon as any one accepts it. This is the standard "tx blasting" technique
// for fastest possible on-chain inclusion — the transaction executes only once.
func (c *Client) blastSendTransaction(ctx context.Context, tx *solana.Transaction, opts rpc.TransactionOpts) (solana.Signature, error) {
	c.mu.RLock()
	endpoints := make([]*rpc.Client, len(c.rpcAll))
	copy(endpoints, c.rpcAll)
	c.mu.RUnlock()

	type result struct {
		sig solana.Signature
		err error
	}

	ch := make(chan result, len(endpoints))
	for _, ep := range endpoints {
		ep := ep
		go func() {
			sig, err := ep.SendTransactionWithOpts(ctx, tx, opts)
			ch <- result{sig: sig, err: err}
		}()
	}

	var lastErr error
	for range endpoints {
		r := <-ch
		if r.err == nil {
			return r.sig, nil
		}
		lastErr = r.err
	}

	return solana.Signature{}, lastErr
}

// GetWalletAddress returns the bot's wallet address
func (c *Client) GetWalletAddress() string {
	return c.wallet.PublicKey().String()
}

// GetSOLBalance returns the wallet's SOL balance
func (c *Client) GetSOLBalance(ctx context.Context) (float64, error) {
	var balance *rpc.GetBalanceResult
	err := c.withRPCFailover(func(r *rpc.Client) error {
		resp, err := r.GetBalance(ctx, c.wallet.PublicKey(), rpc.CommitmentConfirmed)
		if err != nil {
			return err
		}
		balance = resp
		return nil
	})
	if err != nil {
		return 0, err
	}
	return float64(balance.Value) / 1e9, nil
}

// GetTokenBalance returns balance of a specific token
func (c *Client) GetTokenBalance(ctx context.Context, tokenMint string) (float64, int, error) {
	mint := solana.MustPublicKeyFromBase58(tokenMint)

	ata, _, err := solana.FindAssociatedTokenAddress(c.wallet.PublicKey(), mint)
	if err != nil {
		return 0, 0, err
	}

	var resp *rpc.GetTokenAccountBalanceResult
	err = c.withRPCFailover(func(r *rpc.Client) error {
		res, err := r.GetTokenAccountBalance(ctx, ata, rpc.CommitmentConfirmed)
		if err != nil {
			return err
		}
		resp = res
		return nil
	})
	if err != nil || resp == nil || resp.Value == nil {
		return 0, 0, nil
	}

	amount := float64(0)
	fmt.Sscanf(resp.Value.Amount, "%f", &amount)
	amount = amount / float64(pow10(resp.Value.Decimals))

	return amount, int(resp.Value.Decimals), nil
}

// GetTokenInfo fetches token metadata
func (c *Client) GetTokenInfo(ctx context.Context, tokenMint string) (*types.Token, error) {
	mint := solana.MustPublicKeyFromBase58(tokenMint)

	var info *rpc.GetAccountInfoResult
	err := c.withRPCFailover(func(r *rpc.Client) error {
		res, err := r.GetAccountInfo(ctx, mint)
		if err != nil {
			return err
		}
		info = res
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get token info: %w", err)
	}

	if info.Value == nil {
		return nil, fmt.Errorf("token not found")
	}

	data := info.Value.Data.GetBinary()
	if len(data) < 82 {
		return nil, fmt.Errorf("invalid mint data")
	}

	decimals := int(data[44])
	supply := uint64(0)
	for i := 0; i < 8; i++ {
		supply |= uint64(data[36+i]) << (i * 8)
	}

	// Fetch symbol/name from two sources in parallel — use whichever responds first
	symbol, name := c.fetchTokenMetadata(ctx, tokenMint)

	return &types.Token{
		Address:      tokenMint,
		Symbol:       symbol,
		Name:         name,
		Decimals:     decimals,
		Supply:       supply,
		DiscoveredAt: time.Now(),
	}, nil
}

// fetchTokenMetadata fires requests to Jupiter and DexScreener simultaneously
// and returns whichever gives a valid result first.
func (c *Client) fetchTokenMetadata(ctx context.Context, tokenMint string) (string, string) {
	type meta struct {
		symbol, name string
		ok           bool
	}

	ch := make(chan meta, 2)

	go func() {
		sym, name := c.fetchFromJupiterList(ctx, tokenMint)
		ch <- meta{sym, name, sym != "UNKNOWN"}
	}()
	go func() {
		sym, name := c.fetchFromDexScreener(ctx, tokenMint)
		ch <- meta{sym, name, sym != "UNKNOWN"}
	}()

	for i := 0; i < 2; i++ {
		r := <-ch
		if r.ok {
			return r.symbol, r.name
		}
	}
	return "UNKNOWN", "Unknown Token"
}

// fetchFromJupiterList tries Jupiter's strict token list for symbol/name.
func (c *Client) fetchFromJupiterList(ctx context.Context, tokenMint string) (string, string) {
	url := fmt.Sprintf("https://token.jup.ag/strict?mint=%s", tokenMint)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := c.fastClient.Do(req)
	if err != nil {
		return "UNKNOWN", "Unknown Token"
	}
	defer resp.Body.Close()

	var tokens []struct {
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil || len(tokens) == 0 {
		return "UNKNOWN", "Unknown Token"
	}
	return tokens[0].Symbol, tokens[0].Name
}

func (c *Client) fetchFromDexScreener(ctx context.Context, tokenMint string) (string, string) {
	url := fmt.Sprintf("https://api.dexscreener.com/latest/dex/tokens/%s", tokenMint)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := c.fastClient.Do(req)
	if err != nil {
		return "UNKNOWN", "Unknown Token"
	}
	defer resp.Body.Close()

	var result struct {
		Pairs []struct {
			BaseToken struct {
				Symbol string `json:"symbol"`
				Name   string `json:"name"`
			} `json:"baseToken"`
		} `json:"pairs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Pairs) == 0 {
		return "UNKNOWN", "Unknown Token"
	}
	return result.Pairs[0].BaseToken.Symbol, result.Pairs[0].BaseToken.Name
}

// GetTokenPrice returns current price in USD using the fast (3s timeout) client.
func (c *Client) GetTokenPrice(ctx context.Context, tokenMint string) (float64, error) {
	url := fmt.Sprintf("https://price.jup.ag/v6/price?ids=%s", tokenMint)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := c.fastClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Data map[string]struct {
			Price float64 `json:"price"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if data, ok := result.Data[tokenMint]; ok {
		return data.Price, nil
	}
	return 0, fmt.Errorf("price not found")
}

// GetSOLPrice returns current SOL price in USD
func (c *Client) GetSOLPrice(ctx context.Context) (float64, error) {
	return c.GetTokenPrice(ctx, WSOL)
}

// SendTransaction signs and sends a transaction, blasting it to all configured endpoints.
func (c *Client) SendTransaction(ctx context.Context, txBase64 string) (string, error) {
	txBytes, err := base64.StdEncoding.DecodeString(txBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode transaction: %w", err)
	}

	tx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(txBytes))
	if err != nil {
		return "", fmt.Errorf("failed to parse transaction: %w", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(c.wallet.PublicKey()) {
			return &c.wallet
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	opts := rpc.TransactionOpts{
		SkipPreflight:       true,
		PreflightCommitment: rpc.CommitmentProcessed,
	}

	sig, err := c.blastSendTransaction(ctx, tx, opts)
	if err != nil {
		return "", fmt.Errorf("failed to send: %w", err)
	}

	return sig.String(), nil
}

// WaitForConfirmation waits for tx confirmation using WebSocket first (event-driven,
// no polling delay), falling back to HTTP polling if WebSocket is unavailable.
func (c *Client) WaitForConfirmation(ctx context.Context, signature string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if c.cfg.SolanaWSURL != "" {
		if err := c.waitForConfirmationWS(ctx, signature); err == nil {
			return nil
		} else if ctx.Err() != nil {
			return fmt.Errorf("confirmation timeout")
		}
		c.log.Debug("WS confirmation unavailable, falling back to polling")
	}

	return c.waitForConfirmationPoll(ctx, signature)
}

// waitForConfirmationWS subscribes to a transaction signature via WebSocket.
// Returns as soon as the signature is confirmed — no polling delay.
func (c *Client) waitForConfirmationWS(ctx context.Context, signature string) error {
	wsClient, err := ws.Connect(ctx, c.cfg.SolanaWSURL)
	if err != nil {
		return fmt.Errorf("ws connect: %w", err)
	}
	defer wsClient.Close()

	sig := solana.MustSignatureFromBase58(signature)
	sub, err := wsClient.SignatureSubscribe(sig, rpc.CommitmentConfirmed)
	if err != nil {
		return fmt.Errorf("ws subscribe: %w", err)
	}
	defer sub.Unsubscribe()

	result, err := sub.Recv()
	if err != nil {
		return err
	}

	if result.Value.Err != nil {
		return fmt.Errorf("transaction failed: %v", result.Value.Err)
	}
	return nil
}

// waitForConfirmationPoll polls for tx confirmation every 500ms (fallback path).
func (c *Client) waitForConfirmationPoll(ctx context.Context, signature string) error {
	sig := solana.MustSignatureFromBase58(signature)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("confirmation timeout")
		case <-ticker.C:
			var status *rpc.GetSignatureStatusesResult
			err := c.withRPCFailover(func(r *rpc.Client) error {
				res, err := r.GetSignatureStatuses(ctx, false, sig)
				if err != nil {
					return err
				}
				status = res
				return nil
			})
			if err != nil {
				continue
			}
			if len(status.Value) > 0 && status.Value[0] != nil {
				if status.Value[0].Err != nil {
					return fmt.Errorf("transaction failed: %v", status.Value[0].Err)
				}
				if status.Value[0].ConfirmationStatus == rpc.ConfirmationStatusConfirmed ||
					status.Value[0].ConfirmationStatus == rpc.ConfirmationStatusFinalized {
					return nil
				}
			}
		}
	}
}

func pow10(n uint8) uint64 {
	result := uint64(1)
	for i := uint8(0); i < n; i++ {
		result *= 10
	}
	return result
}
