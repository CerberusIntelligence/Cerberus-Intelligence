package telegram

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"solana-trading-bot/config"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"github.com/sirupsen/logrus"
)

// MTProtoClient handles Telegram MTProto connection for channel monitoring
type MTProtoClient struct {
	cfg         *config.Config
	log         *logrus.Logger
	monitor     *Monitor
	client      *telegram.Client
	dispatcher  tg.UpdateDispatcher
	gaps        *updates.Manager
	sessionFile string
}

// NewMTProtoClient creates a new MTProto client
func NewMTProtoClient(cfg *config.Config, log *logrus.Logger, monitor *Monitor) *MTProtoClient {
	return &MTProtoClient{
		cfg:         cfg,
		log:         log,
		monitor:     monitor,
		sessionFile: "mtproto_session.json",
	}
}

// Connect establishes connection to Telegram using MTProto
func (m *MTProtoClient) Connect(ctx context.Context) error {
	if m.cfg.TelegramAPIID == 0 || m.cfg.TelegramAPIHash == "" {
		m.log.Warn("Telegram API ID/Hash not configured - channel monitoring disabled")
		m.log.Warn("You can still receive signals via:")
		m.log.Warn("  1. Forward messages to your bot")
		m.log.Warn("  2. Use /check <token_address> command")
		return nil
	}

	m.log.Info("Initializing MTProto client for channel monitoring...")

	// Setup session storage
	sessionStorage := &session.FileStorage{
		Path: m.sessionFile,
	}

	// Setup update dispatcher
	m.dispatcher = tg.NewUpdateDispatcher()

	// Create gaps manager for handling updates
	m.gaps = updates.New(updates.Config{
		Handler: m.dispatcher,
	})

	// Create client
	m.client = telegram.NewClient(m.cfg.TelegramAPIID, m.cfg.TelegramAPIHash, telegram.Options{
		SessionStorage: sessionStorage,
		UpdateHandler:  m.gaps,
	})

	// Register message handler
	m.dispatcher.OnNewChannelMessage(m.handleChannelMessage)
	m.dispatcher.OnNewMessage(m.handleMessage)

	// Run client
	go func() {
		if err := m.client.Run(ctx, func(ctx context.Context) error {
			// Check if already authorized
			status, err := m.client.Auth().Status(ctx)
			if err != nil {
				return fmt.Errorf("auth status: %w", err)
			}

			if !status.Authorized {
				m.log.Info("Not authorized, starting authentication flow...")
				if err := m.authenticate(ctx); err != nil {
					return fmt.Errorf("authentication failed: %w", err)
				}
			}

			// Get self info
			self, err := m.client.Self(ctx)
			if err != nil {
				return fmt.Errorf("get self: %w", err)
			}
			m.log.WithFields(logrus.Fields{
				"id":       self.ID,
				"username": self.Username,
			}).Info("MTProto authenticated")

			// Resolve and join monitored channels
			if err := m.setupChannels(ctx); err != nil {
				m.log.WithError(err).Warn("Failed to setup some channels")
			}

			// Start gaps recovery
			if err := m.gaps.Run(ctx, m.client.API(), self.ID, updates.AuthOptions{}); err != nil {
				m.log.WithError(err).Debug("Gaps recovery skipped")
			}

			m.log.Info("MTProto channel monitoring active")

			// Keep running
			<-ctx.Done()
			return ctx.Err()
		}); err != nil {
			m.log.WithError(err).Error("MTProto client error")
		}
	}()

	return nil
}

// authenticate handles the phone number authentication flow
func (m *MTProtoClient) authenticate(ctx context.Context) error {
	// Check if we have saved phone number
	phone := m.cfg.TelegramPhone
	if phone == "" {
		fmt.Print("Enter your phone number (with country code, e.g., +1234567890): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		phone = strings.TrimSpace(input)
	}

	// Create auth flow
	flow := auth.NewFlow(
		terminalAuth{phone: phone},
		auth.SendCodeOptions{},
	)

	if err := m.client.Auth().IfNecessary(ctx, flow); err != nil {
		return err
	}

	return nil
}

// terminalAuth implements auth.UserAuthenticator for terminal input
type terminalAuth struct {
	phone string
}

func (t terminalAuth) Phone(_ context.Context) (string, error) {
	return t.phone, nil
}

func (t terminalAuth) Password(_ context.Context) (string, error) {
	fmt.Print("Enter 2FA password (if enabled): ")
	reader := bufio.NewReader(os.Stdin)
	password, _ := reader.ReadString('\n')
	return strings.TrimSpace(password), nil
}

func (t terminalAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	fmt.Print("Enter the code sent to your Telegram: ")
	reader := bufio.NewReader(os.Stdin)
	code, _ := reader.ReadString('\n')
	return strings.TrimSpace(code), nil
}

func (t terminalAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign up not supported")
}

func (t terminalAuth) AcceptTermsOfService(_ context.Context, _ tg.HelpTermsOfService) error {
	return nil
}

// setupChannels resolves and joins the monitored channels
func (m *MTProtoClient) setupChannels(ctx context.Context) error {
	api := m.client.API()

	for _, channelName := range m.cfg.MonitoredChannels {
		// Clean channel name
		channelName = strings.TrimPrefix(channelName, "@")
		channelName = strings.TrimSpace(channelName)

		if channelName == "" {
			continue
		}

		m.log.WithField("channel", channelName).Debug("Resolving channel...")

		// Resolve username to get channel info
		resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
			Username: channelName,
		})
		if err != nil {
			m.log.WithError(err).WithField("channel", channelName).Warn("Failed to resolve channel")
			continue
		}

		// Find channel in results
		for _, chat := range resolved.Chats {
			switch c := chat.(type) {
			case *tg.Channel:
				m.log.WithFields(logrus.Fields{
					"channel": c.Title,
					"id":      c.ID,
				}).Info("Monitoring channel")

				// Try to join if not already a member
				_, err := api.ChannelsJoinChannel(ctx, &tg.InputChannel{
					ChannelID:  c.ID,
					AccessHash: c.AccessHash,
				})
				if err != nil {
					// Ignore "already participant" errors
					if !strings.Contains(err.Error(), "USER_ALREADY_PARTICIPANT") {
						m.log.WithError(err).Debug("Could not join channel (may already be member)")
					}
				}
			}
		}
	}

	return nil
}

// handleChannelMessage processes new channel messages
func (m *MTProtoClient) handleChannelMessage(ctx context.Context, e tg.Entities, update *tg.UpdateNewChannelMessage) error {
	msg, ok := update.Message.(*tg.Message)
	if !ok || msg.Message == "" {
		return nil
	}

	// Get channel info
	channelName := "unknown"
	if peer, ok := msg.PeerID.(*tg.PeerChannel); ok {
		if channel, ok := e.Channels[peer.ChannelID]; ok {
			channelName = channel.Username
			if channelName == "" {
				channelName = channel.Title
			}
		}
	}

	// Check if this channel is in our monitored list
	if !m.isMonitoredChannel(channelName) {
		return nil
	}

	m.log.WithFields(logrus.Fields{
		"channel": channelName,
		"msgID":   msg.ID,
	}).Debug("New message from monitored channel")

	// Process the message
	m.monitor.ProcessMessage(channelName, msg.ID, msg.Message)

	return nil
}

// handleMessage processes regular messages (for groups)
func (m *MTProtoClient) handleMessage(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
	msg, ok := update.Message.(*tg.Message)
	if !ok || msg.Message == "" {
		return nil
	}

	// Get chat info
	chatName := "unknown"
	switch peer := msg.PeerID.(type) {
	case *tg.PeerChat:
		if chat, ok := e.Chats[peer.ChatID]; ok {
			chatName = chat.Title
		}
	case *tg.PeerUser:
		// Direct messages - skip
		return nil
	}

	// Check if monitored
	if !m.isMonitoredChannel(chatName) {
		return nil
	}

	m.log.WithFields(logrus.Fields{
		"chat":  chatName,
		"msgID": msg.ID,
	}).Debug("New message from monitored chat")

	m.monitor.ProcessMessage(chatName, msg.ID, msg.Message)

	return nil
}

// isMonitoredChannel checks if channel is in the monitored list
func (m *MTProtoClient) isMonitoredChannel(name string) bool {
	name = strings.ToLower(strings.TrimPrefix(name, "@"))

	// If no channels configured, monitor all
	if len(m.cfg.MonitoredChannels) == 0 {
		return true
	}

	for _, ch := range m.cfg.MonitoredChannels {
		ch = strings.ToLower(strings.TrimPrefix(ch, "@"))
		if ch == name {
			return true
		}
	}
	return false
}

// SimulateSignal allows manual testing by simulating a signal
func (m *MTProtoClient) SimulateSignal(channelName string, messageID int, text string) {
	m.monitor.ProcessMessage(channelName, messageID, text)
}

// ProcessForwardedMessage handles messages forwarded to the bot
func (m *MTProtoClient) ProcessForwardedMessage(fromChannel string, text string) {
	// Check if from a monitored channel
	for _, ch := range m.cfg.MonitoredChannels {
		ch = strings.TrimPrefix(ch, "@")
		if strings.EqualFold(ch, fromChannel) {
			m.log.WithField("channel", fromChannel).Info("Processing forwarded message")
			m.monitor.ProcessMessage(fromChannel, 0, text)
			return
		}
	}

	// Process anyway if channels list is empty (monitor all)
	if len(m.cfg.MonitoredChannels) == 0 {
		m.monitor.ProcessMessage(fromChannel, 0, text)
	}
}

// GetChannelStatus returns monitoring status
func (m *MTProtoClient) GetChannelStatus() string {
	if m.cfg.TelegramAPIID == 0 {
		return "Monitoring via forwarded messages only"
	}
	return fmt.Sprintf("MTProto active - Monitoring %d channels", len(m.cfg.MonitoredChannels))
}

// SaveSession saves auth session info
func (m *MTProtoClient) SaveSession() error {
	// Session is auto-saved by FileStorage
	return nil
}

// LoadSession loads saved session
func (m *MTProtoClient) LoadSession() error {
	if _, err := os.Stat(m.sessionFile); os.IsNotExist(err) {
		return fmt.Errorf("no session file found")
	}
	return nil
}

// SessionInfo holds saved session data
type SessionInfo struct {
	Phone     string    `json:"phone"`
	AuthDate  time.Time `json:"auth_date"`
	SessionID string    `json:"session_id"`
}

// GetSessionInfo returns current session info
func (m *MTProtoClient) GetSessionInfo() (*SessionInfo, error) {
	data, err := os.ReadFile(m.sessionFile)
	if err != nil {
		return nil, err
	}

	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		// Session file exists but different format - that's OK
		return &SessionInfo{
			AuthDate: time.Now(),
		}, nil
	}

	return &info, nil
}

// Disconnect closes the MTProto connection
func (m *MTProtoClient) Disconnect() error {
	// Client will disconnect when context is cancelled
	return nil
}

// EnsureSessionDir creates the session directory if needed
func EnsureSessionDir() error {
	dir := filepath.Dir("mtproto_session.json")
	if dir != "." && dir != "" {
		return os.MkdirAll(dir, 0700)
	}
	return nil
}
