package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
)

type Update struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

type Message struct {
	Chat Chat   `json:"chat"`
	Text string `json:"text"`
}

type Chat struct {
	ID int64 `json:"id"`
}

// GetUpdates long-polls Telegram for new messages since offset.
func (c *Client) GetUpdates(offset int) ([]Update, error) {
	u := fmt.Sprintf("%s%s/getUpdates?timeout=30&offset=%d", apiBase, c.token, offset)
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.Result, nil
}

// Reply sends a plain-text reply to a specific chat.
func (c *Client) Reply(chatID int64, text string) error {
	u := fmt.Sprintf("%s%s/sendMessage", apiBase, c.token)
	resp, err := c.http.PostForm(u, url.Values{
		"chat_id":    {strconv.FormatInt(chatID, 10)},
		"text":       {text},
		"parse_mode": {"Markdown"},
	})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// SetMyCommands registers the command list shown in Telegram's "/" menu.
func (c *Client) SetMyCommands() error {
	commands := `[
		{"command":"help","description":"Show all available commands"},
		{"command":"run","description":"Scan for top wallets and send results here"},
		{"command":"list","description":"Show wallets currently being copy-traded"},
		{"command":"add","description":"Add a wallet: /add <address>"},
		{"command":"remove","description":"Remove a wallet: /remove <address>"}
	]`
	u := fmt.Sprintf("%s%s/setMyCommands", apiBase, c.token)
	resp, err := c.http.PostForm(u, url.Values{"commands": {commands}})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
