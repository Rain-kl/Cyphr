// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/config"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	defaultLimit   = 200
	maxLimit       = 500
	maxPageSize    = 100
	hoursInDay     = 24
	analyticsDays  = 7
	topActiveLimit = 10
)

// logsResponse 历史日志查询响应
type logsResponse struct {
	Lines      []logger.LogEntry `json:"lines"`
	HasMore    bool              `json:"has_more"`
	NextCursor int               `json:"next_cursor"` // 用于加载更早日志的 cursor
}

// GetLogs 获取历史日志
// @Summary 获取系统日志
// @Description 分页获取系统历史日志，cursor=0 获取最新日志，cursor>0 获取更早日志
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param cursor query int false "日志游标，0=获取最新" default(0)
// @Param limit query int false "每页条数" default(200)
// @Success 200 {object} response.Any{data=logsResponse} "日志列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/logs [get]
func GetLogs(c *gin.Context) {
	cursorStr := c.DefaultQuery("cursor", "0")
	limitStr := c.DefaultQuery("limit", "200")

	var cursor, limit int
	if _, err := parsePositiveInt(cursorStr, &cursor); err != nil {
		response.AbortWithError(c, http.StatusBadRequest, InvalidCursorParam)
		return
	}
	if _, err := parsePositiveInt(limitStr, &limit); err != nil || limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	entries, hasMore := logger.GlobalRingBuffer.Query(cursor, limit)

	resp := logsResponse{
		Lines:   entries,
		HasMore: hasMore,
	}
	if len(entries) > 0 {
		resp.NextCursor = entries[0].Index
	}

	c.JSON(http.StatusOK, response.OK(resp))
}

// wsMessage WebSocket 消息格式
type wsMessage struct {
	Type string          `json:"type"` // "log" | "error"
	Data json.RawMessage `json:"data"`
}

// HandleLogWebSocket WebSocket 端点，实时推送系统日志
// @Summary 系统日志实时推送
// @Description 通过 WebSocket 实时推送系统日志，需要管理员权限
// @Tags admin
// @Router /api/v1/admin/logs/ws [get]
func HandleLogWebSocket(c *gin.Context) {
	upgrader := getUpgrader()

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	ch := logger.GlobalRingBuffer.Subscribe()
	defer logger.GlobalRingBuffer.Unsubscribe(ch)

	done := make(chan struct{})
	util.Go(func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})

	for {
		select {
		case <-done:
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(entry)
			msg := wsMessage{Type: "log", Data: data}
			payload, _ := json.Marshal(msg)
			if err := conn.WriteMessage(1, payload); err != nil {
				return
			}
		}
	}
}

// accessLogItem 访问日志单条数据
type accessLogItem struct {
	ID        uint64 `json:"id,string"`
	TraceID   string `json:"trace_id"`
	UserID    uint64 `json:"user_id,string"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Path      string `json:"path"`
	Method    string `json:"method"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	Headers   string `json:"headers"`
	Status    int32  `json:"status"`
	Latency   int64  `json:"latency"`
	CreatedAt string `json:"created_at"`
}

// accessLogsResponse 访问日志查询响应
type accessLogsResponse struct {
	Total uint64          `json:"total"`
	List  []accessLogItem `json:"list"`
}

func buildAccessLogFilter(ctx context.Context, c *gin.Context) (contracts.AccessLogFilterDTO, error) {
	filter := contracts.AccessLogFilterDTO{}

	username := c.Query("username")
	if username != "" {
		var userIDs []uint64
		gormDB := GetDB(ctx)
		if gormDB != nil {
			if err := gormDB.Table("w_users").
				Where("username LIKE ? ESCAPE '\\'", "%"+util.EscapeLike(username)+"%").
				Pluck("id", &userIDs).Error; err != nil {
				return filter, fmt.Errorf("查询用户信息失败: %w", err)
			}
		}
		filter.UserIDs = userIDs
	}

	if path := c.Query("path"); path != "" {
		filter.Path = path
	}

	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := parseAccessLogTime(startTime); err == nil {
			filter.StartTime = &t
		}
	}

	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := parseAccessLogTime(endTime); err == nil {
			filter.EndTime = &t
		}
	}

	return filter, nil
}

func parseAccessLogTime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", value)
}

func enrichAccessLogsWithUsers(ctx context.Context, list []accessLogItem) {
	if len(list) == 0 {
		return
	}

	userIDs := make([]uint64, 0, len(list))
	seen := make(map[uint64]struct{}, len(list))
	for _, item := range list {
		if _, ok := seen[item.UserID]; ok {
			continue
		}
		seen[item.UserID] = struct{}{}
		userIDs = append(userIDs, item.UserID)
	}

	userMap := make(map[uint64]struct{ Username, Nickname string })
	var users []struct {
		ID       uint64
		Username string
		Nickname string
	}
	gormDB := GetDB(ctx)
	if gormDB != nil {
		if err := gormDB.Table("w_users").Where("id IN ?", userIDs).Find(&users).Error; err == nil {
			for _, u := range users {
				userMap[u.ID] = struct{ Username, Nickname string }{Username: u.Username, Nickname: u.Nickname}
			}
		}
	}
	for i := range list {
		if info, ok := userMap[list[i].UserID]; ok {
			list[i].Username = info.Username
			list[i].Nickname = info.Nickname
		}
	}
}

// GetAccessLogs 获取 ClickHouse 异步采集的访问日志
// @Summary 获取用户访问日志
// @Description 分页并按照用户、接口路径、时间范围等维度检索用户访问日志列表（需要管理员权限）
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param username query string false "用户名模糊搜索"
// @Param path query string false "接口路径模糊搜索"
// @Param start_time query string false "起始时间（RFC3339 或 YYYY-MM-DD HH:MM:SS）"
// @Param end_time query string false "结束时间（RFC3339 或 YYYY-MM-DD HH:MM:SS）"
// @Success 200 {object} response.Any{data=accessLogsResponse} "访问日志列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/logs/access [get]
func GetAccessLogs(c *gin.Context) {
	ctx := c.Request.Context()
	rc := GetRiskControlService()
	if rc == nil {
		response.AbortInternal(c, "日志存储服务未初始化")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	filter, err := buildAccessLogFilter(ctx, c)
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if filter.UserIDs != nil && len(filter.UserIDs) == 0 {
		c.JSON(http.StatusOK, response.OK(accessLogsResponse{Total: 0, List: []accessLogItem{}}))
		return
	}

	logs, total, err := rc.QueryAccessLogs(ctx, filter, page, pageSize)
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if total == 0 {
		c.JSON(http.StatusOK, response.OK(accessLogsResponse{Total: 0, List: []accessLogItem{}}))
		return
	}

	list := make([]accessLogItem, len(logs))
	for i, logItem := range logs {
		list[i] = accessLogItem{
			ID:        logItem.ID,
			UserID:    logItem.UserID,
			Path:      logItem.Path,
			Method:    logItem.Method,
			IP:        logItem.IP,
			UserAgent: logItem.UserAgent,
			Status:    logItem.Status,
			Latency:   logItem.Latency,
			CreatedAt: logItem.CreatedAt.Format(time.RFC3339),
		}
	}
	enrichAccessLogsWithUsers(ctx, list)

	c.JSON(http.StatusOK, response.OK(accessLogsResponse{
		Total: total,
		List:  list,
	}))
}

// trendItem 趋势图数据点
type trendItem struct {
	Date  string `json:"date"`
	Count uint64 `json:"count"`
}

// browserItem 浏览器占比排行
type browserItem struct {
	Browser string `json:"browser"`
	Count   uint64 `json:"count"`
}

// topUserItem 活跃用户数据
type topUserItem struct {
	UserID   uint64 `json:"user_id,string"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Count    uint64 `json:"count"`
}

// logsAnalyticsResponse 访问日志数据分析结果
type logsAnalyticsResponse struct {
	Trend    []trendItem   `json:"trend"`
	Browsers []browserItem `json:"browsers"`
	TopUsers []topUserItem `json:"top_users"`
}

// GetLogsAnalytics 获取 ClickHouse 访问日志图表聚合指标
// @Summary 获取访问日志分析数据
// @Description 聚合统计最近 7 天的每日访问趋势、浏览器分布以及前 10 名最活跃用户排行（需要管理员权限）
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=logsAnalyticsResponse} "分析统计数据"
// @Failure 500 {object} response.Any "内部错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/logs/analytics [get]
func GetLogsAnalytics(c *gin.Context) {
	ctx := c.Request.Context()
	rc := GetRiskControlService()
	if rc == nil {
		response.AbortInternal(c, "日志存储服务未初始化")
		return
	}

	stats, err := rc.QueryAccessLogStats(ctx, analyticsDays)
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, "查询访问趋势失败: "+err.Error())
		return
	}
	trendList := make([]trendItem, len(stats))
	for i, st := range stats {
		trendList[i] = trendItem{
			Date:  st.Date,
			Count: st.PV,
		}
	}

	browserList := []browserItem{}
	topUsers := []topUserItem{}

	c.JSON(http.StatusOK, response.OK(logsAnalyticsResponse{
		Trend:    trendList,
		Browsers: browserList,
		TopUsers: topUsers,
	}))
}

func getUpgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}

			// 1. 同源检查 (Same-origin check)
			u, err := url.Parse(origin)
			if err == nil && strings.EqualFold(u.Host, r.Host) {
				return true
			}

			// 2. 检查配置的允许跨域 Origin (Check allowed origins in system config)
			ctx := r.Context()
			if sc, err := GetSystemConfigByKey(ctx, ConfigKeyServerAddress); err == nil && sc.Value != "" {
				originToCheck := strings.TrimRight(strings.TrimSpace(origin), "/")
				allowedOrigins := strings.Split(sc.Value, ",")
				for _, allowed := range allowedOrigins {
					allowed = strings.TrimRight(strings.TrimSpace(allowed), "/")
					if allowed != "" && strings.EqualFold(allowed, originToCheck) {
						return true
					}
				}
			}
			return false
		},
	}
}

func parsePositiveInt(s string, result *int) (bool, error) {
	if s == "" {
		*result = 0
		return true, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return false, err
	}
	*result = n
	return true, nil
}

// Log DB Switch Task
const (
	// LogDBSwitchTask 切换日志数据库任务标识。
	LogDBSwitchTask = "logs:db_switch"
	// TaskTypeLogDBSwitch 管理端任务类型。
	TaskTypeLogDBSwitch = "logs_db_switch"

	copyBatchSize    = 1000
	targetPostgres   = "postgres"
	targetSQLite     = "sqlite"
	targetClickHouse = "clickhouse"
)

// LogDBSwitchMeta 描述切换日志数据库任务。
var LogDBSwitchMeta = contracts.TaskMetaDTO{
	Name:        LogDBSwitchTask,
	DisplayName: "切换日志数据库",
	Description: "复制迁移用户访问日志并在成功后切换日志主库（期间禁止日志写入）",
	MaxRetry:    3,
	Queue:       "default",
	Params: []contracts.TaskParamDTO{
		{Name: "target", Description: "迁移目标：postgres（主库为 PG 时）、sqlite（主库为 SQLite 时）或 clickhouse", Type: "string", Required: true},
	},
}

type logDBSwitchPayload struct {
	Target string `json:"target"`
}

// LogDBSwitchHandler 切换日志数据库任务处理器。
type LogDBSwitchHandler struct{}

// ValidatePayload 校验并规范化参数。
func (h *LogDBSwitchHandler) ValidatePayload(payload []byte) ([]byte, error) {
	var p logDBSwitchPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}
	p.Target = normalizeTarget(p.Target)
	if !validTarget(p.Target) {
		return nil, fmt.Errorf("目标日志库不合法: %s", p.Target)
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeTarget(v string) string {
	switch v {
	case targetPostgres, "postgresql":
		return targetPostgres
	case targetSQLite, "sqlite3":
		return targetSQLite
	case targetClickHouse, "ch":
		return targetClickHouse
	}
	return v
}

func validTarget(v string) bool {
	return v == targetPostgres || v == targetSQLite || v == targetClickHouse
}

// Execute 执行迁移。
func (h *LogDBSwitchHandler) Execute(ctx context.Context, payload []byte) (*contracts.TaskResultDTO, error) {
	var p logDBSwitchPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}
	p.Target = normalizeTarget(p.Target)
	if err := validateSwitch(ctx, p.Target); err != nil {
		return nil, err
	}

	source, err := currentLogDatabase(ctx)
	if err != nil {
		return nil, err
	}

	taskSvc := GetTaskService()
	if taskSvc != nil {
		taskSvc.AppendLog(ctx, "开始切换日志数据库：%s -> %s", source, p.Target)
	}

	if err := setMigrationFlag(ctx, "migrating"); err != nil {
		return nil, err
	}
	defer func() {
		if err := setMigrationFlag(ctx, ""); err != nil {
			logger.ErrorF(ctx, "清除日志迁移冻结标记失败: %v", err)
		}
	}()

	rc := GetRiskControlService()
	if rc != nil {
		if err := rc.SwitchLogEngine(ctx, p.Target); err != nil {
			return nil, err
		}
	}

	if err := flipLogDatabase(ctx, p.Target); err != nil {
		return nil, err
	}

	if taskSvc != nil {
		taskSvc.AppendLog(ctx, "日志数据库已切换为 %s，写入恢复", p.Target)
	}
	return &contracts.TaskResultDTO{Message: fmt.Sprintf("日志数据库已从 %s 切换为 %s", source, p.Target)}, nil
}

func validateSwitch(ctx context.Context, target string) error {
	source, err := currentLogDatabase(ctx)
	if err != nil {
		return err
	}
	if source == target {
		return errors.New("目标日志库与当前日志库相同，无需迁移")
	}
	switch target {
	case targetClickHouse:
		if !config.Config.ClickHouse.Enabled {
			return errors.New("ClickHouse 未启用，无法迁移到 ClickHouse")
		}
	case targetPostgres:
		if !config.Config.Database.Enabled {
			return errors.New("PostgreSQL 未启用（当前主库为 SQLite），无法迁移到 PostgreSQL")
		}
	case targetSQLite:
		if config.Config.Database.Enabled {
			return errors.New("当前主库为 PostgreSQL，日志库不能设置为 SQLite")
		}
	}
	return nil
}

func currentLogDatabase(ctx context.Context) (string, error) {
	cfg, err := GetSystemConfigByKey(ctx, ConfigKeyLogDatabase)
	if err != nil {
		return "", fmt.Errorf("读取日志主库失败: %w", err)
	}
	if cfg.Value == "" {
		return "", errors.New("日志主库配置为空")
	}
	return cfg.Value, nil
}

func setMigrationFlag(ctx context.Context, v string) error {
	return SaveOrUpdateSystemConfig(ctx, ConfigKeyLogDBMigration, v)
}

func flipLogDatabase(ctx context.Context, target string) error {
	return SaveOrUpdateSystemConfig(ctx, ConfigKeyLogDatabase, target)
}
