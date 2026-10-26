package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"unicode/utf8"
)

// 智商探针状态与目标模式常量。
const (
	ModelIQProbeStatusPass        = "pass"
	ModelIQProbeStatusWrongAnswer = "wrong_answer"
	ModelIQProbeStatusError       = "error"

	ModelIQProbeTargetModeAccountModel = "account_model"
	ModelIQProbeTargetModeModelOnly    = "model_only"

	// modelIQProbeMinIntervalMinutes / Max：探测间隔边界（5 分钟 ~ 7 天）。
	modelIQProbeMinIntervalMinutes = 5
	modelIQProbeMaxIntervalMinutes = 10080

	// modelIQProbeRetentionDays 结果保留天数。
	modelIQProbeRetentionDays = 30
	// modelIQProbeActualAnswerMaxBytes 回答原文存档上限（超长截断）。
	modelIQProbeActualAnswerMaxBytes = 4 << 10
	// modelIQProbeErrorMessageMaxBytes 错误信息存档上限。
	modelIQProbeErrorMessageMaxBytes = 2000
	// modelIQProbeAnswerExcerptMaxRunes 用户端回答摘要长度。
	modelIQProbeAnswerExcerptMaxRunes = 120

	// modelIQProbeProbeConcurrency 单轮探测并发上限。
	modelIQProbeProbeConcurrency = 3
	// modelIQProbeRecentLimit 用户端汇总拉取的最近结果条数上限。
	modelIQProbeRecentLimit = 500
	// modelIQProbeRecentPerModel 用户端每模型展示的最近结果数（时间线条带深度，
	// 与渠道监控卡片时间线的 60 点规格一致）。
	modelIQProbeRecentPerModel = 60
)

// 智商探针错误定义。
var (
	ErrModelIQProbeInvalidConfig = infraerrors.BadRequest(
		"MODEL_IQ_INVALID_CONFIG",
		"model IQ probe config is invalid",
	)
	ErrModelIQProbeResultNotFound = infraerrors.NotFound(
		"MODEL_IQ_RESULT_NOT_FOUND",
		"model IQ probe result not found",
	)
	ErrModelIQProbeRunInProgress = infraerrors.Conflict(
		"MODEL_IQ_RUN_IN_PROGRESS",
		"a model IQ probe cycle is already running",
	)
	ErrModelIQProbeUnavailable = infraerrors.ServiceUnavailable(
		"MODEL_IQ_UNAVAILABLE",
		"model IQ probe is not available",
	)
)

// modelIQProbeNumberRegex 提取响应中的独立整数 token（与 challenge 判分同款）。
var modelIQProbeNumberRegex = regexp.MustCompile(`-?\d+`)

// DefaultModelIQProbeQuestion 默认糖果问题题面（标准答案 21）。
// 降智模型常给出 20/23/29 等忽略"形状靠手感可分辨"的答案，以此判别。
const DefaultModelIQProbeQuestion = `在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。现已知不同口味的糖和不同形状的数量统计如下表。参赛者需要在活动前决定摸出的糖果数目，那么，最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（同时手中有圆形苹果味匹配五角星桃子味糖果，或者有圆形桃子味匹配五角星苹果味糖果都满足要求）

苹果味 桃子味 西瓜味
圆形 7 9 8
五角星形 7 6 4`

// ModelIQProbeConfig 智商探针配置（settings 表 JSON 存储）。
type ModelIQProbeConfig struct {
	Enabled         bool     `json:"enabled"`
	Question        string   `json:"question"`
	ExpectedAnswer  string   `json:"expected_answer"`
	IntervalMinutes int      `json:"interval_minutes"`
	TargetMode      string   `json:"target_mode"`
	Models          []string `json:"models"`
	AccountIDs      []int64  `json:"account_ids"`
}

// DefaultModelIQProbeConfig 返回默认配置（糖果问题，答案 21，每 6 小时一轮）。
func DefaultModelIQProbeConfig() *ModelIQProbeConfig {
	return &ModelIQProbeConfig{
		Enabled:         false,
		Question:        DefaultModelIQProbeQuestion,
		ExpectedAnswer:  "21",
		IntervalMinutes: 360,
		TargetMode:      ModelIQProbeTargetModeAccountModel,
		Models:          []string{},
		AccountIDs:      []int64{},
	}
}

// DecodeModelIQProbeConfig 解析 settings 中的配置 JSON；缺省字段回落到默认值。
func DecodeModelIQProbeConfig(raw string) *ModelIQProbeConfig {
	cfg := DefaultModelIQProbeConfig()
	if strings.TrimSpace(raw) == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		slog.Warn("model_iq_probe: decode config failed, fallback to defaults", "error", err)
		return DefaultModelIQProbeConfig()
	}
	cfg.normalize()
	return cfg
}

// normalize 补齐空缺字段并清理输入。
func (c *ModelIQProbeConfig) normalize() {
	if strings.TrimSpace(c.Question) == "" {
		c.Question = DefaultModelIQProbeQuestion
	}
	if c.IntervalMinutes <= 0 {
		c.IntervalMinutes = 360
	}
	if c.TargetMode != ModelIQProbeTargetModeModelOnly {
		c.TargetMode = ModelIQProbeTargetModeAccountModel
	}
	c.Models = trimAndDedupStrings(c.Models)
	c.AccountIDs = trimAndDedupInt64s(c.AccountIDs)
}

// Validate 校验配置。
func (c *ModelIQProbeConfig) Validate() error {
	if strings.TrimSpace(c.Question) == "" {
		return ErrModelIQProbeInvalidConfig.WithCause(fmt.Errorf("question is required"))
	}
	if strings.TrimSpace(c.ExpectedAnswer) == "" {
		return ErrModelIQProbeInvalidConfig.WithCause(fmt.Errorf("expected_answer is required"))
	}
	if len(c.ExpectedAnswer) > 200 {
		return ErrModelIQProbeInvalidConfig.WithCause(fmt.Errorf("expected_answer too long"))
	}
	if c.IntervalMinutes < modelIQProbeMinIntervalMinutes || c.IntervalMinutes > modelIQProbeMaxIntervalMinutes {
		return ErrModelIQProbeInvalidConfig.WithCause(fmt.Errorf("interval_minutes must be in [%d, %d]", modelIQProbeMinIntervalMinutes, modelIQProbeMaxIntervalMinutes))
	}
	if c.TargetMode != ModelIQProbeTargetModeAccountModel && c.TargetMode != ModelIQProbeTargetModeModelOnly {
		return ErrModelIQProbeInvalidConfig.WithCause(fmt.Errorf("target_mode must be account_model or model_only"))
	}
	return nil
}

// Interval 返回探测间隔时长。
func (c *ModelIQProbeConfig) Interval() time.Duration {
	return time.Duration(c.IntervalMinutes) * time.Minute
}

// ---------- DTO 与仓储端口 ----------

// ModelIQProbeResult 单次智商探针结果。
// account_name 仅供管理端展示；用户端汇总绝不输出账号信息。
type ModelIQProbeResult struct {
	ID              int64     `json:"id"`
	AccountID       *int64    `json:"account_id"`
	AccountName     string    `json:"account_name"`
	Model           string    `json:"model"`
	Protocol        string    `json:"protocol"`
	Status          string    `json:"status"`
	OverrideStatus  *string   `json:"override_status"`
	EffectiveStatus string    `json:"effective_status"`
	ErrorCategory   string    `json:"error_category"`
	ErrorMessage    string    `json:"error_message"`
	ExpectedAnswer  string    `json:"expected_answer"`
	ActualAnswer    string    `json:"actual_answer"`
	LatencyMs       int64     `json:"latency_ms"`
	InputTokens     *int      `json:"input_tokens"`
	OutputTokens    *int      `json:"output_tokens"`
	CacheReadTokens *int      `json:"cache_read_tokens"`
	CreatedAt       time.Time `json:"created_at"`
}

// ResolveEffectiveStatus 返回生效状态：人工改判优先于自动判定。
func (r *ModelIQProbeResult) ResolveEffectiveStatus() string {
	if r == nil {
		return ""
	}
	if r.OverrideStatus != nil && *r.OverrideStatus != "" {
		return *r.OverrideStatus
	}
	return r.Status
}

// ModelIQProbeListFilters 管理端结果过滤条件。
type ModelIQProbeListFilters struct {
	Model     string // 精确匹配；空 = 全部
	AccountID int64  // 0 = 全部
	Status    string // 生效状态过滤；空 = 全部
}

// ModelIQProbeRepository 智商探针结果仓储端口。
type ModelIQProbeRepository interface {
	Insert(ctx context.Context, result *ModelIQProbeResult) error
	ListPage(ctx context.Context, page, pageSize int, filters ModelIQProbeListFilters) ([]*ModelIQProbeResult, int64, error)
	ListRecent(ctx context.Context, limit int) ([]*ModelIQProbeResult, error)
	OverrideStatus(ctx context.Context, id int64, status *string) error
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// ---------- 用户端汇总 DTO ----------

// ModelIQProbeStatusPoint 时间线上的一个判定点。
type ModelIQProbeStatusPoint struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
}

// ModelIQProbeUserSummary 单个模型的探针汇总（用户端）。
type ModelIQProbeUserSummary struct {
	Model  string                    `json:"model"`
	Latest *ModelIQProbeUserLatest   `json:"latest"`
	Recent []ModelIQProbeStatusPoint `json:"recent"`
}

// ModelIQProbeUserLatest 最新一次探测结果（含回答摘要）。
type ModelIQProbeUserLatest struct {
	Status        string    `json:"status"`
	AnswerExcerpt string    `json:"answer_excerpt,omitempty"`
	CheckedAt     time.Time `json:"checked_at"`
}

// ModelIQProbeCycleSummary 一轮探测的汇总。
type ModelIQProbeCycleSummary struct {
	Total       int `json:"total"`
	Pass        int `json:"pass"`
	WrongAnswer int `json:"wrong_answer"`
	Error       int `json:"error"`
}

// ---------- 判分 ----------

// ValidateModelIQAnswer 按"出现即通过"判分：
//   - 期望答案可解析为整数时，提取响应中的独立整数 token，出现即通过
//     （与 channel_monitor_challenge 的 validateChallenge 同语义）；
//   - 否则退化为大小写不敏感的包含匹配。
//
// 已知宽松边界：如"不是 21 而是 20"这类回答会被判 pass（产品选定语义），
// 管理端可通过人工改判纠正。
func ValidateModelIQAnswer(responseText, expected string) bool {
	expected = strings.TrimSpace(expected)
	if responseText == "" || expected == "" {
		return false
	}
	if _, err := strconv.Atoi(expected); err == nil {
		for _, m := range modelIQProbeNumberRegex.FindAllString(responseText, -1) {
			if m == expected {
				return true
			}
		}
		return false
	}
	return strings.Contains(strings.ToLower(responseText), strings.ToLower(expected))
}

// ---------- Service ----------

// ModelIQProbeService 智商探针业务：配置、目标解析、探测执行、结果查询。
type ModelIQProbeService struct {
	repo               ModelIQProbeRepository
	settings           *SettingService
	accountRepo        AccountRepository
	accountTestService *AccountTestService

	running atomic.Bool
}

// NewModelIQProbeService 创建服务。
func NewModelIQProbeService(
	repo ModelIQProbeRepository,
	settings *SettingService,
	accountRepo AccountRepository,
	accountTestService *AccountTestService,
) *ModelIQProbeService {
	return &ModelIQProbeService{
		repo:               repo,
		settings:           settings,
		accountRepo:        accountRepo,
		accountTestService: accountTestService,
	}
}

// GetConfig 读取配置（缺省回落默认值）。
func (s *ModelIQProbeService) GetConfig(ctx context.Context) (*ModelIQProbeConfig, error) {
	cfg := DefaultModelIQProbeConfig()
	if s.settings != nil {
		raw, err := s.settings.GetRawSettingValue(ctx, SettingKeyModelIQProbeConfig)
		if err == nil {
			cfg = DecodeModelIQProbeConfig(raw)
		} else if !ErrSettingNotFound.Is(err) {
			return nil, err
		}
	}
	return cfg, nil
}

// SaveConfig 校验并保存配置。
func (s *ModelIQProbeService) SaveConfig(ctx context.Context, cfg *ModelIQProbeConfig) error {
	if cfg == nil {
		return ErrModelIQProbeInvalidConfig.WithCause(fmt.Errorf("config is required"))
	}
	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if s.settings == nil {
		return ErrModelIQProbeUnavailable
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.settings.SetRawSettingValue(ctx, SettingKeyModelIQProbeConfig, string(payload))
}

// ---------- 目标解析 ----------

type modelIQProbeTarget struct {
	account *Account
	model   string
}

// listProbeAccounts 返回参与探测的账号集合（已按 ID 升序，保证确定性）。
func (s *ModelIQProbeService) listProbeAccounts(ctx context.Context, cfg *ModelIQProbeConfig) ([]*Account, error) {
	if s.accountRepo == nil {
		return nil, nil
	}
	accounts, err := s.accountRepo.ListSchedulable(ctx)
	if err != nil {
		return nil, err
	}
	selected := make([]*Account, 0, len(accounts))
	for i := range accounts {
		a := &accounts[i]
		if !a.IsOpenAI() || !a.IsActive() {
			continue
		}
		if len(cfg.AccountIDs) > 0 && !slices.Contains(cfg.AccountIDs, a.ID) {
			continue
		}
		selected = append(selected, a)
	}
	slices.SortFunc(selected, func(a, b *Account) int { return int(a.ID - b.ID) })
	return selected, nil
}

// resolveProbeModels 返回要探测的模型集合：配置指定或从账号上游目录自动发现。
func (s *ModelIQProbeService) resolveProbeModels(ctx context.Context, cfg *ModelIQProbeConfig, accounts []*Account) []string {
	if len(cfg.Models) > 0 {
		return cfg.Models
	}
	return s.discoverGPTModels(ctx, accounts)
}

// discoverGPTModels 并发拉取各账号的模型目录，取 gpt-* 文本模型并集。
// 单账号失败只记日志，不影响其他账号。
func (s *ModelIQProbeService) discoverGPTModels(ctx context.Context, accounts []*Account) []string {
	seen := make(map[string]struct{})
	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(modelIQProbeProbeConcurrency)
	for _, a := range accounts {
		account := a
		group.Go(func() error {
			models, err := s.accountTestService.FetchOpenAIAccountModels(groupCtx, account)
			if err != nil {
				slog.Warn("model_iq_probe: discover models failed", "account_id", account.ID, "error", err)
				return nil
			}
			mu.Lock()
			for _, m := range models {
				id := strings.ToLower(strings.TrimSpace(m.ID))
				if !strings.HasPrefix(id, "gpt-") || strings.HasPrefix(id, "gpt-image-") {
					continue
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
			}
			mu.Unlock()
			return nil
		})
	}
	_ = group.Wait()
	discovered := make([]string, 0, len(seen))
	for id := range seen {
		discovered = append(discovered, id)
	}
	slices.Sort(discovered)
	return discovered
}

// resolveTargets 按 target_mode 组装（账号, 模型）目标列表。
func (s *ModelIQProbeService) resolveTargets(ctx context.Context, cfg *ModelIQProbeConfig) ([]modelIQProbeTarget, error) {
	accounts, err := s.listProbeAccounts(ctx, cfg)
	if err != nil {
		return nil, err
	}
	models := s.resolveProbeModels(ctx, cfg, accounts)
	if len(models) == 0 || len(accounts) == 0 {
		return nil, nil
	}

	targets := make([]modelIQProbeTarget, 0, len(accounts)*len(models))
	if cfg.TargetMode == ModelIQProbeTargetModeModelOnly {
		for _, model := range models {
			for _, a := range accounts {
				if a.IsModelSupported(model) {
					targets = append(targets, modelIQProbeTarget{account: a, model: model})
					break
				}
			}
		}
		return targets, nil
	}
	for _, a := range accounts {
		for _, model := range models {
			if a.IsModelSupported(model) {
				targets = append(targets, modelIQProbeTarget{account: a, model: model})
			}
		}
	}
	return targets, nil
}

// ---------- 探测执行 ----------

// RunProbeCycle 执行一轮探测：解析目标 → 并发探测 → 写入结果。
// 进程内防重入；配置未启用时直接返回空汇总。
func (s *ModelIQProbeService) RunProbeCycle(ctx context.Context) (*ModelIQProbeCycleSummary, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return &ModelIQProbeCycleSummary{}, nil
	}
	if !s.running.CompareAndSwap(false, true) {
		return nil, ErrModelIQProbeRunInProgress
	}
	defer s.running.Store(false)

	targets, err := s.resolveTargets(ctx, cfg)
	if err != nil {
		return nil, err
	}
	summary := &ModelIQProbeCycleSummary{Total: len(targets)}
	if len(targets) == 0 {
		slog.Info("model_iq_probe: no probe targets resolved")
		return summary, nil
	}

	latestRows := make([]*ModelIQProbeResult, len(targets))
	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(modelIQProbeProbeConcurrency)
	for i, t := range targets {
		idx, target := i, t
		group.Go(func() error {
			result := s.probeTarget(groupCtx, target, cfg)
			latestRows[idx] = result
			if err := s.repo.Insert(groupCtx, result); err != nil {
				slog.Error("model_iq_probe: insert result failed", "model", target.model, "error", err)
				return nil
			}
			mu.Lock()
			switch result.ResolveEffectiveStatus() {
			case ModelIQProbeStatusPass:
				summary.Pass++
			case ModelIQProbeStatusWrongAnswer:
				summary.WrongAnswer++
			default:
				summary.Error++
			}
			mu.Unlock()
			return nil
		})
	}
	_ = group.Wait()
	slog.Info("model_iq_probe: cycle completed", "total", summary.Total, "pass", summary.Pass, "wrong_answer", summary.WrongAnswer, "error", summary.Error)
	return summary, nil
}

// probeTarget 对单个（账号, 模型）目标执行一次糖果问题探测。
// 复刻 AccountTestService.RunAccountTestBackground 的内存 SSE 通道，
// 以便拿到回答原文与耗时。
func (s *ModelIQProbeService) probeTarget(ctx context.Context, target modelIQProbeTarget, cfg *ModelIQProbeConfig) *ModelIQProbeResult {
	startedAt := time.Now()

	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = (&http.Request{}).WithContext(ctx)

	testErr := s.accountTestService.TestAccountConnectionWithAccount(ginCtx, target.account, target.model, cfg.Question, AccountTestModeDefault)
	responseText, errMsg := parseTestSSEOutput(w.Body.String())

	result := &ModelIQProbeResult{
		AccountID:      &target.account.ID,
		AccountName:    target.account.Name,
		Model:          target.model,
		Protocol:       inferModelIQProbeProtocol(target.account),
		ExpectedAnswer: cfg.ExpectedAnswer,
		CreatedAt:      startedAt,
	}
	result.LatencyMs = time.Since(startedAt).Milliseconds()

	switch {
	case testErr != nil || strings.TrimSpace(responseText) == "":
		result.Status = ModelIQProbeStatusError
		message := strings.TrimSpace(errMsg)
		if message == "" && testErr != nil {
			message = testErr.Error()
		}
		if message == "" {
			message = "empty response"
		}
		result.ErrorMessage = truncateStringBytes(message, modelIQProbeErrorMessageMaxBytes)
	case ValidateModelIQAnswer(responseText, cfg.ExpectedAnswer):
		result.Status = ModelIQProbeStatusPass
		result.ActualAnswer = truncateStringBytes(responseText, modelIQProbeActualAnswerMaxBytes)
	default:
		result.Status = ModelIQProbeStatusWrongAnswer
		result.ActualAnswer = truncateStringBytes(responseText, modelIQProbeActualAnswerMaxBytes)
	}
	return result
}

// inferModelIQProbeProtocol 按账号类型推断探测协议（仅供排查参考）。
func inferModelIQProbeProtocol(account *Account) string {
	if account != nil && account.IsOpenAIOAuthLike() {
		return "responses"
	}
	return "chat_completions"
}

// ---------- 查询 ----------

// ListResultsAdmin 管理端分页查询。
func (s *ModelIQProbeService) ListResultsAdmin(ctx context.Context, page, pageSize int, filters ModelIQProbeListFilters) ([]*ModelIQProbeResult, int64, error) {
	if s.repo == nil {
		return nil, 0, ErrModelIQProbeUnavailable
	}
	return s.repo.ListPage(ctx, page, pageSize, filters)
}

// UserSummary 用户端按模型汇总（最新判定 + 最近时间线）。
// 探针未启用或无数据时返回空列表；绝不包含账号信息。
func (s *ModelIQProbeService) UserSummary(ctx context.Context) ([]*ModelIQProbeUserSummary, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled || s.repo == nil {
		return []*ModelIQProbeUserSummary{}, nil
	}
	rows, err := s.repo.ListRecent(ctx, modelIQProbeRecentLimit)
	if err != nil {
		return nil, err
	}
	return userSummaryFromRows(rows), nil
}

// userSummaryFromRows 把按 created_at desc 排序的结果行按模型聚合成用户端视图。
// 不输出账号信息；回答摘要只取每模型最新一行。
func userSummaryFromRows(rows []*ModelIQProbeResult) []*ModelIQProbeUserSummary {
	latestRow := make(map[string]*ModelIQProbeResult)
	byModel := make(map[string]*ModelIQProbeUserSummary)
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		item, ok := byModel[row.Model]
		if !ok {
			item = &ModelIQProbeUserSummary{Model: row.Model, Recent: []ModelIQProbeStatusPoint{}}
			byModel[row.Model] = item
			order = append(order, row.Model)
		}
		if _, ok := latestRow[row.Model]; !ok {
			latestRow[row.Model] = row
		}
		if len(item.Recent) < modelIQProbeRecentPerModel {
			item.Recent = append(item.Recent, ModelIQProbeStatusPoint{Status: row.ResolveEffectiveStatus(), CheckedAt: row.CreatedAt})
		}
	}

	items := make([]*ModelIQProbeUserSummary, 0, len(order))
	for _, model := range order {
		item := byModel[model]
		// Recent 当前是新→旧，时间线展示转为旧→新。
		slices.Reverse(item.Recent)
		if row := latestRow[model]; row != nil {
			item.Latest = &ModelIQProbeUserLatest{
				Status:        row.ResolveEffectiveStatus(),
				AnswerExcerpt: truncateStringRunes(row.ActualAnswer, modelIQProbeAnswerExcerptMaxRunes),
				CheckedAt:     row.CreatedAt,
			}
		}
		items = append(items, item)
	}
	slices.SortFunc(items, func(a, b *ModelIQProbeUserSummary) int { return strings.Compare(a.Model, b.Model) })
	return items
}

// OverrideStatus 人工改判；status 传 nil 表示清除改判恢复自动判定。
func (s *ModelIQProbeService) OverrideStatus(ctx context.Context, id int64, status *string) error {
	if status != nil {
		switch *status {
		case ModelIQProbeStatusPass, ModelIQProbeStatusWrongAnswer, ModelIQProbeStatusError:
		default:
			return ErrModelIQProbeInvalidConfig.WithCause(fmt.Errorf("invalid override status"))
		}
	}
	return s.repo.OverrideStatus(ctx, id, status)
}

// CleanupOldResults 清理超过保留期的结果行（OpsCleanupService 每日调用）。
func (s *ModelIQProbeService) CleanupOldResults(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -modelIQProbeRetentionDays)
	deleted, err := s.repo.DeleteBefore(ctx, cutoff)
	if err != nil {
		return err
	}
	if deleted > 0 {
		slog.Info("model_iq_probe: cleanup old results", "deleted", deleted)
	}
	return nil
}

// ---------- 工具 ----------

func trimAndDedupStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func trimAndDedupInt64s(in []int64) []int64 {
	out := make([]int64, 0, len(in))
	seen := make(map[int64]struct{}, len(in))
	for _, v := range in {
		if v <= 0 {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// truncateStringBytes 按字节截断并保证 UTF-8 合法。
func truncateStringBytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := s[:maxBytes]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

// truncateStringRunes 按字符数截断并去掉首尾空白。
func truncateStringRunes(s string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes])
}
