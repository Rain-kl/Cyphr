// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package telegram implements the Telegram private-chat adapter.
package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/backend/plugins/domain/message_gateway"
	tele "gopkg.in/telebot.v4"
)

// Adapter is a Telegram private-chat channel.
type Adapter struct {
	cfg       message_gateway.ChannelConfig
	onInbound message_gateway.Handler
	bot       *tele.Bot
}

// New constructs a Telegram adapter. Call message_gateway.Register from the runner.
func New(cfg message_gateway.ChannelConfig, onInbound message_gateway.Handler) (message_gateway.Channel, error) {
	if strings.TrimSpace(cfg.Credentials["bot_token"]) == "" {
		return nil, fmt.Errorf("telegram: bot_token is required")
	}
	return &Adapter{cfg: cfg, onInbound: onInbound}, nil
}

// Type returns telegram.
func (a *Adapter) Type() string { return message_gateway.ChannelTypeTelegram }

// Capabilities reports private-chat media support.
func (a *Adapter) Capabilities() message_gateway.Capability {
	return message_gateway.Capability{Text: true, Image: true, File: true, Reply: true}
}

// Connect starts long polling.
func (a *Adapter) Connect(ctx context.Context) error {
	pref := tele.Settings{
		Token:  a.cfg.Credentials["bot_token"],
		Poller: &tele.LongPoller{Timeout: 10},
	}
	if base := strings.TrimSpace(a.cfg.Extra["base_url"]); base != "" {
		pref.URL = strings.TrimSuffix(base, "/")
	}
	bot, err := tele.NewBot(pref)
	if err != nil {
		return fmt.Errorf("telegram: new bot: %w", err)
	}
	a.bot = bot
	bot.Handle(tele.OnText, func(c tele.Context) error {
		a.handleTeleMessage(ctx, c.Message())
		return nil
	})
	bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		a.handleTeleMessage(ctx, c.Message())
		return nil
	})
	bot.Handle(tele.OnDocument, func(c tele.Context) error {
		a.handleTeleMessage(ctx, c.Message())
		return nil
	})
	go bot.Start()
	go func() {
		<-ctx.Done()
		bot.Stop()
	}()
	return nil
}

// Disconnect stops the bot.
func (a *Adapter) Disconnect(_ context.Context) error {
	if a.bot != nil {
		a.bot.Stop()
	}
	return nil
}

// Send replies to a private chat.
func (a *Adapter) Send(_ context.Context, to message_gateway.Recipient, msg message_gateway.OutboundMessage) error {
	if a.bot == nil {
		return fmt.Errorf("telegram: not connected")
	}
	chatID, err := strconv.ParseInt(to.ChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: chat id: %w", err)
	}
	_, err = a.bot.Send(tele.ChatID(chatID), msg.Text)
	return err
}

func (a *Adapter) handleTeleMessage(ctx context.Context, m *tele.Message) {
	if m == nil || m.Chat == nil || m.Chat.Type != tele.ChatPrivate {
		return
	}
	if a.onInbound == nil {
		return
	}
	msg := message_gateway.InboundMessage{
		ChannelID:      a.cfg.ID,
		ChannelType:    message_gateway.ChannelTypeTelegram,
		PlatformUserID: strconv.FormatInt(m.Sender.ID, 10),
		ChatID:         strconv.FormatInt(m.Chat.ID, 10),
		MessageID:      strconv.Itoa(m.ID),
		Text:           m.Text,
	}
	if m.Caption != "" && msg.Text == "" {
		msg.Text = m.Caption
	}
	if a.bot != nil {
		msg.Attachments = a.downloadMedia(m)
	}
	_ = a.onInbound(ctx, msg)
}

func (a *Adapter) downloadMedia(m *tele.Message) []message_gateway.Attachment {
	var files []*tele.File
	var names []string
	if m.Photo != nil {
		files = append(files, m.Photo.MediaFile())
		names = append(names, "photo.jpg")
	}
	if m.Document != nil {
		files = append(files, &m.Document.File)
		name := m.Document.FileName
		if name == "" {
			name = "file"
		}
		names = append(names, name)
	}
	if len(files) == 0 {
		return nil
	}
	dir, err := os.MkdirTemp("", "wg-tg-*")
	if err != nil {
		return []message_gateway.Attachment{{Error: err.Error()}}
	}
	out := make([]message_gateway.Attachment, 0, len(files))
	for i, f := range files {
		path := filepath.Join(dir, names[i])
		if err := a.bot.Download(f, path); err != nil {
			out = append(out, message_gateway.Attachment{FileName: names[i], Error: err.Error()})
			continue
		}
		out = append(out, message_gateway.Attachment{Path: path, FileName: names[i]})
	}
	return out
}
