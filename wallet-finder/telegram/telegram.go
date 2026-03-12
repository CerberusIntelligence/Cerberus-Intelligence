package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const apiBase = "https://api.telegram.org/bot"

type Client struct {
	token  string
	chatID string
	http   *http.Client
}

func NewClient(token, chatID string) *Client {
	return &Client{
		token:  token,
		chatID: chatID,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Send(text string) error {
	url := fmt.Sprintf("%s%s/sendMessage", apiBase, c.token)
	payload := map[string]string{
		"chat_id":    c.chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	body, _ := json.Marshal(payload)
	resp, err := c.http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram send failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram HTTP %d", resp.StatusCode)
	}
	return nil
}

// SendChunkedTo sends a chunked message to a specific chat ID.
func (c *Client) SendChunkedTo(chatID int64, text string) error {
	saved := c.chatID
	c.chatID = fmt.Sprintf("%d", chatID)
	err := c.SendChunked(text)
	c.chatID = saved
	return err
}

// SendChunked splits long messages so they stay under Telegram's 4096-char limit.
func (c *Client) SendChunked(text string) error {
	const maxLen = 4000
	for len(text) > 0 {
		chunk := text
		if len(chunk) > maxLen {
			// Split at last newline before limit
			cut := maxLen
			for cut > 0 && text[cut] != '\n' {
				cut--
			}
			if cut == 0 {
				cut = maxLen
			}
			chunk = text[:cut]
			text = text[cut:]
		} else {
			text = ""
		}
		if err := c.Send(chunk); err != nil {
			return err
		}
		if len(text) > 0 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}
