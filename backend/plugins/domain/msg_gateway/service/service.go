// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service implements domain business logic and channel runners for msg_gateway.
package service

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Handler processes one inbound message.
type Handler func(ctx context.Context, msg do.InboundMessage) error

// Factory constructs a Channel from decrypted config.
type Factory func(cfg do.ChannelConfig, onInbound Handler) (Channel, error)

// Channel is one connected messaging adapter.
type Channel interface {
	Type() string
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Send(ctx context.Context, to do.Recipient, msg do.OutboundMessage) error
	Capabilities() do.Capability
}

var (
	factoriesMu sync.RWMutex
	factories   = map[string]Factory{}
)

// Register stores a channel factory under typ.
func Register(typ string, fn Factory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[typ] = fn
}

// Lookup returns a previously registered factory.
func Lookup(typ string) (Factory, bool) {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	fn, ok := factories[typ]
	return fn, ok
}

// Re-exported constants.
const (
	CodeAlphabet = consts.CodeAlphabet
	CodeLength   = consts.CodeLength
)

// GenerateCode returns an 8-character pairing code.
func GenerateCode() (string, error) {
	buf := make([]byte, CodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, CodeLength)
	for i, b := range buf {
		out[i] = CodeAlphabet[int(b)%len(CodeAlphabet)]
	}
	return string(out), nil
}

// NormalizeCode strips separators and uppercases.
func NormalizeCode(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '-' || unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

// FormatCode renders ABCD-EFGH.
func FormatCode(s string) string {
	s = NormalizeCode(s)
	if len(s) != CodeLength {
		return s
	}
	return s[:4] + "-" + s[4:]
}

var (
	credentialSecretMu sync.RWMutex
	credentialSecret   string
)

// SetCredentialSecret sets the secret used to derive CredentialKey.
func SetCredentialSecret(secret string) {
	credentialSecretMu.Lock()
	defer credentialSecretMu.Unlock()
	credentialSecret = secret
}

// CredentialKey is AES-256 hex derived from the session secret.
func CredentialKey() string {
	credentialSecretMu.RLock()
	secret := credentialSecret
	credentialSecretMu.RUnlock()
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// EncryptCredentials encrypts a credential map as JSON.
func EncryptCredentials(creds map[string]string) (string, error) {
	if creds == nil {
		creds = map[string]string{}
	}
	raw, err := json.Marshal(creds)
	if err != nil {
		return "", err
	}
	return util.Encrypt(CredentialKey(), string(raw))
}

// DecryptCredentials decrypts a credential map.
func DecryptCredentials(ciphertext string) (map[string]string, error) {
	if ciphertext == "" {
		return map[string]string{}, nil
	}
	plain, err := util.Decrypt(CredentialKey(), ciphertext)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(plain), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}

// ParseExtra decodes optional extra JSON into a string map.
func ParseExtra(raw string) map[string]string {
	if raw == "" {
		return map[string]string{}
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return map[string]string{}
	}
	return out
}

// EncodeExtra encodes extra fields as JSON.
func EncodeExtra(extra map[string]string) string {
	if extra == nil {
		return ""
	}
	raw, err := json.Marshal(extra)
	if err != nil {
		return ""
	}
	return string(raw)
}

// Runner manages lifecycle for long-lived channel adapters (WebSocket, long-polling, etc.).
type Runner struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

// GlobalRunner is the default global runner instance.
var GlobalRunner = &Runner{}

// Start starts all background long-lived channel runners.
func Start(ctx context.Context) error {
	GlobalRunner.mu.Lock()
	defer GlobalRunner.mu.Unlock()

	if GlobalRunner.running {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	GlobalRunner.cancel = cancel
	GlobalRunner.running = true

	logger.InfoF(runCtx, "[MessageGateway] Starting bot channel runners...")
	return nil
}

// Stop stops the channel runner.
func Stop() {
	GlobalRunner.mu.Lock()
	defer GlobalRunner.mu.Unlock()

	if !GlobalRunner.running {
		return
	}

	if GlobalRunner.cancel != nil {
		GlobalRunner.cancel()
	}
	GlobalRunner.running = false
}

// Cordis contract singletons consumed by service layer.
var (
	cacheMu  sync.RWMutex
	cacheSvc contracts.CacheService
	taskMu   sync.RWMutex
	taskSvc  contracts.TaskService
	userMu   sync.RWMutex
	userSvc  contracts.UserService
)

// SetCacheService sets the cache service.
func SetCacheService(s contracts.CacheService) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheSvc = s
}

// SetTaskService sets the task service.
func SetTaskService(s contracts.TaskService) {
	taskMu.Lock()
	defer taskMu.Unlock()
	taskSvc = s
}

// SetUserService sets the user service.
func SetUserService(s contracts.UserService) {
	userMu.Lock()
	defer userMu.Unlock()
	userSvc = s
}

// GetCache resolves the cache service for the context.
func GetCache(ctx context.Context) contracts.CacheService {
	if s, err := core.InjectFrom[contracts.CacheService](ctx); err == nil && s != nil {
		return s
	}
	cacheMu.RLock()
	s := cacheSvc
	cacheMu.RUnlock()
	return s
}

// GetTaskService returns the task service.
func GetTaskService(ctx context.Context) contracts.TaskService {
	if s, err := core.InjectFrom[contracts.TaskService](ctx); err == nil && s != nil {
		return s
	}
	taskMu.RLock()
	defer taskMu.RUnlock()
	return taskSvc
}

// GetUserService resolves the user service for the context.
func GetUserService(ctx context.Context) contracts.UserService {
	if s, err := core.InjectFrom[contracts.UserService](ctx); err == nil && s != nil {
		return s
	}
	userMu.RLock()
	s := userSvc
	userMu.RUnlock()
	return s
}

// BindChannel consumes a pairing code and binds the platform identity to the user.
func BindChannel(ctx context.Context, userID uint64, req do.BindRequest) (do.BindingDTO, error) {
	channelID, err := strconv.ParseUint(strings.TrimSpace(req.ChannelID), 10, 64)
	if err != nil || channelID == 0 {
		return do.BindingDTO{}, consts.ErrChannelIDRequired
	}
	code := NormalizeCode(req.Code)
	if code == "" {
		return do.BindingDTO{}, consts.ErrCodeInvalid
	}
	pairing, err := dao.GetPairingCode(ctx, code)
	if err != nil {
		if errors.Is(err, consts.ErrRecordNotFound) {
			return do.BindingDTO{}, consts.ErrCodeInvalid
		}
		return do.BindingDTO{}, err
	}
	if !pairing.ExpiresAt.After(time.Now()) {
		return do.BindingDTO{}, consts.ErrCodeInvalid
	}
	if pairing.ChannelID != channelID {
		return do.BindingDTO{}, consts.ErrChannelMismatch
	}
	ch, err := dao.GetMessageChannel(ctx, channelID)
	if err != nil {
		if errors.Is(err, consts.ErrRecordNotFound) {
			return do.BindingDTO{}, consts.ErrCodeInvalid
		}
		return do.BindingDTO{}, err
	}
	if !ch.Enabled {
		return do.BindingDTO{}, consts.ErrChannelDisabled
	}

	existing, err := dao.GetBindingByChannelPlatform(ctx, channelID, pairing.PlatformUserID)
	if err != nil && !errors.Is(err, consts.ErrRecordNotFound) {
		return do.BindingDTO{}, err
	}
	if err == nil && existing != nil {
		if existing.UserID != userID {
			return do.BindingDTO{}, consts.ErrPlatformAlreadyBound
		}
		_ = dao.DeletePairingCode(ctx, pairing.Code)
		return ToBindingDTO(existing, ch), nil
	}

	row := &entity.MessageBinding{
		UserID:         userID,
		ChannelID:      channelID,
		PlatformUserID: pairing.PlatformUserID,
	}
	if err := dao.CreateMessageBinding(ctx, row); err != nil {
		return do.BindingDTO{}, err
	}
	if err := dao.DeletePairingCode(ctx, pairing.Code); err != nil {
		return do.BindingDTO{}, err
	}
	return ToBindingDTO(row, ch), nil
}

// ListEnabledPublicChannels returns the channels a user may bind to.
func ListEnabledPublicChannels(ctx context.Context) ([]do.PublicChannelDTO, error) {
	rows, err := dao.ListEnabledMessageChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]do.PublicChannelDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, do.PublicChannelDTO{ID: row.ID, Name: row.Name, Type: row.Type})
	}
	return out, nil
}

// ListUserBindings returns the binding rows of one user enriched with channel info.
func ListUserBindings(ctx context.Context, userID uint64) ([]do.BindingDTO, error) {
	rows, err := dao.ListBindingsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]do.BindingDTO, 0, len(rows))
	for i := range rows {
		ch, err := dao.GetMessageChannel(ctx, rows[i].ChannelID)
		if err != nil {
			out = append(out, ToBindingDTO(&rows[i], nil))
			continue
		}
		out = append(out, ToBindingDTO(&rows[i], ch))
	}
	return out, nil
}

// UnbindChannel removes a binding owned by the given user.
func UnbindChannel(ctx context.Context, userID, bindingID uint64) error {
	row, err := dao.GetMessageBinding(ctx, bindingID)
	if err != nil {
		if errors.Is(err, consts.ErrRecordNotFound) {
			return consts.ErrBindingNotFound
		}
		return err
	}
	if row.UserID != userID {
		return consts.ErrBindingForbidden
	}
	return dao.DeleteMessageBinding(ctx, bindingID)
}

// ToBindingDTO projects a binding row and its optional channel onto the user DTO.
func ToBindingDTO(row *entity.MessageBinding, ch *entity.MessageChannel) do.BindingDTO {
	dto := do.BindingDTO{
		ID:             row.ID,
		UserID:         row.UserID,
		ChannelID:      row.ChannelID,
		PlatformUserID: row.PlatformUserID,
		CreatedAt:      row.CreatedAt,
	}
	if ch != nil {
		dto.ChannelName = ch.Name
		dto.ChannelType = ch.Type
	}
	return dto
}
