package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestObservabilityDisabledByDefault 覆盖"默认关闭、配置缺失即 no-op"。
func TestObservabilityDisabledByDefault(t *testing.T) {
	cfg := &Config{}
	require.False(t, cfg.Observability.Langfuse.Enabled)
	require.False(t, cfg.Observability.Langfuse.Usable())
	require.NoError(t, cfg.validateObservability())
}

// TestObservabilityUsableRequiresAllFields 覆盖装配条件的完备性判定。
func TestObservabilityUsableRequiresAllFields(t *testing.T) {
	full := LangfuseConfig{Enabled: true, Host: "http://10.0.0.5:3000", PublicKey: "pk", SecretKey: "sk"}
	require.True(t, full.Usable())

	cases := map[string]LangfuseConfig{
		"缺 host":   {Enabled: true, PublicKey: "pk", SecretKey: "sk"},
		"缺 public": {Enabled: true, Host: "http://10.0.0.5:3000", SecretKey: "sk"},
		"缺 secret": {Enabled: true, Host: "http://10.0.0.5:3000", PublicKey: "pk"},
		"开关关闭":     {Enabled: false, Host: "http://10.0.0.5:3000", PublicKey: "pk", SecretKey: "sk"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.False(t, tc.Usable())
		})
	}
}

// TestObservabilityDegradesToNoopInsteadOfFailingStartup 是上生产前最关键的约束：
// 观测层配置不完整或地址非法时只能降级为 no-op，不能把网关启动挡死。
func TestObservabilityDegradesToNoopInsteadOfFailingStartup(t *testing.T) {
	cfg := &Config{}
	cfg.Observability.Langfuse = LangfuseConfig{Enabled: true}
	require.NoError(t, cfg.validateObservability())
	require.False(t, cfg.Observability.Langfuse.Enabled, "缺凭据必须降级为关闭")

	cfg.Observability.Langfuse = LangfuseConfig{
		Enabled: true, Host: "http://10.0.0.5:3000", PublicKey: "pk", SecretKey: "sk",
	}
	require.NoError(t, cfg.validateObservability())
	require.True(t, cfg.Observability.Langfuse.Enabled, "配置齐全时必须保持开启")

	cfg.Observability.Langfuse.Host = "//bad host"
	require.NoError(t, cfg.validateObservability())
	require.False(t, cfg.Observability.Langfuse.Enabled, "非法地址必须降级为关闭")
}

// TestObservabilityQueueClampedWhenCaptureOn 覆盖正文采集下的队列收紧。
func TestObservabilityQueueClampedWhenCaptureOn(t *testing.T) {
	cfg := &Config{}
	cfg.Observability.Langfuse = LangfuseConfig{
		Enabled: true, Host: "http://10.0.0.5:3000", PublicKey: "pk", SecretKey: "sk",
		QueueSize: 4096,
	}
	require.NoError(t, cfg.validateObservability())
	require.Equal(t, 4096, cfg.Observability.Langfuse.QueueSize, "未采正文时不收紧")

	cfg.Observability.Langfuse.CaptureResponse = true
	require.NoError(t, cfg.validateObservability())
	lf := cfg.Observability.Langfuse
	require.LessOrEqual(t, lf.QueueSize, maxObservabilityQueueWithCapture)
	perRecord := lf.CaptureMaxBytes + lf.CapturePromptMaxBytes + 4*1024
	require.LessOrEqual(t, lf.QueueSize*perRecord, observabilityCaptureMemoryBudget,
		"队列驻留必须落在正文字节预算内")
	require.GreaterOrEqual(t, lf.QueueSize, 64, "预算不会把队列压到无法工作")
}

// TestObservabilityDefaultsRegistered 覆盖 setDefaults 里注册的默认值。
func TestObservabilityDefaultsRegistered(t *testing.T) {
	resetViperWithJWTSecret(t)
	setDefaults()

	require.False(t, viper.GetBool("observability.langfuse.enabled"))
	require.Equal(t, 4096, viper.GetInt("observability.langfuse.queue_size"))
	require.Equal(t, 64, viper.GetInt("observability.langfuse.batch_size"))
	require.Equal(t, 5, viper.GetInt("observability.langfuse.flush_interval_seconds"))
	require.Equal(t, 10, viper.GetInt("observability.langfuse.timeout_seconds"))
	require.Equal(t, 300, viper.GetInt("observability.langfuse.turn_idle_seconds"))
}

// TestObservabilityTurnIdleClamped 覆盖轮次空闲阈值的钳制：
// 太小会把一轮切成几张卡，太大则卡片迟迟不出现。
func TestObservabilityTurnIdleClamped(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"未配置用缺省值", 0, 300},
		{"过小抬到下限", 1, 30},
		{"正常值原样保留", 120, 120},
		{"过大压到上限", 99999, 3600},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.Observability.Langfuse = LangfuseConfig{
				Enabled: true, Host: "http://10.0.0.5:3000", PublicKey: "pk", SecretKey: "sk",
				TurnIdleSeconds: tc.in,
			}
			require.NoError(t, cfg.validateObservability())
			require.Equal(t, tc.want, cfg.Observability.Langfuse.TurnIdleSeconds)
		})
	}
}

// TestObservabilityCaptureBytesClamped 覆盖正文上限的钳制与缺省值。
func TestObservabilityCaptureBytesClamped(t *testing.T) {
	base := func() *Config {
		cfg := &Config{}
		cfg.Observability.Langfuse = LangfuseConfig{
			Enabled: true, Host: "http://10.0.0.5:3000", PublicKey: "pk", SecretKey: "sk",
			CapturePrompt: true, CaptureResponse: true,
		}
		return cfg
	}

	cfg := base()
	require.NoError(t, cfg.validateObservability())
	require.Equal(t, 65536, cfg.Observability.Langfuse.CaptureMaxBytes, "未配置时用缺省值")
	require.Equal(t, 524288, cfg.Observability.Langfuse.CapturePromptMaxBytes)

	cfg = base()
	cfg.Observability.Langfuse.CaptureMaxBytes = 1 << 30
	cfg.Observability.Langfuse.CapturePromptMaxBytes = 1 << 30
	require.NoError(t, cfg.validateObservability())
	require.Equal(t, maxCaptureSideBytes, cfg.Observability.Langfuse.CaptureMaxBytes,
		"超过 2MB 的配置必须被钳住，否则一条调用的载荷会撞上 ingestion 的 4.5MB 上限")
	require.Equal(t, maxCaptureSideBytes, cfg.Observability.Langfuse.CapturePromptMaxBytes)

	cfg = base()
	cfg.Observability.Langfuse.CaptureMaxBytes = 100
	require.NoError(t, cfg.validateObservability())
	require.Equal(t, minCaptureSideBytes, cfg.Observability.Langfuse.CaptureMaxBytes, "过小值抬到下限")
}
