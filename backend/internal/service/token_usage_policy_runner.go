package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
)

const tokenUsagePolicyRunnerMaxWorkers = 5

type TokenUsageAutoPolicyRunner struct {
	repo  TokenUsageAutoPolicyRepository
	svc   *TokenUsageAutoPolicyService
	cfg   *config.Config
	cron  *cron.Cron
	start sync.Once
	stop  sync.Once
}

func NewTokenUsageAutoPolicyRunner(repo TokenUsageAutoPolicyRepository, svc *TokenUsageAutoPolicyService, cfg *config.Config) *TokenUsageAutoPolicyRunner {
	return &TokenUsageAutoPolicyRunner{repo: repo, svc: svc, cfg: cfg}
}

func (r *TokenUsageAutoPolicyRunner) Start() {
	if r == nil || r.repo == nil || r.svc == nil {
		return
	}
	r.start.Do(func() {
		loc := time.Local
		if r.cfg != nil && r.cfg.Timezone != "" {
			if parsed, err := time.LoadLocation(r.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}
		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(loc))
		if _, err := c.AddFunc("* * * * *", r.runDuePolicies); err != nil {
			logger.LegacyPrintf("service.token_usage_policy_runner", "[TokenUsagePolicyRunner] not started: %v", err)
			return
		}
		r.cron = c
		r.cron.Start()
		logger.LegacyPrintf("service.token_usage_policy_runner", "[TokenUsagePolicyRunner] started")
	})
}

func (r *TokenUsageAutoPolicyRunner) Stop() {
	if r == nil {
		return
	}
	r.stop.Do(func() {
		if r.cron == nil {
			return
		}
		ctx := r.cron.Stop()
		select {
		case <-ctx.Done():
		case <-time.After(3 * time.Second):
			logger.LegacyPrintf("service.token_usage_policy_runner", "[TokenUsagePolicyRunner] cron stop timed out")
		}
	})
}

func (r *TokenUsageAutoPolicyRunner) runDuePolicies() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	policies, err := r.repo.ListDuePolicies(ctx, time.Now().UTC(), 50)
	if err != nil {
		logger.LegacyPrintf("service.token_usage_policy_runner", "[TokenUsagePolicyRunner] list due policies failed: %v", err)
		return
	}
	if len(policies) == 0 {
		return
	}

	sem := make(chan struct{}, tokenUsagePolicyRunnerMaxWorkers)
	var wg sync.WaitGroup
	for _, policy := range policies {
		sem <- struct{}{}
		wg.Add(1)
		go func(p TokenUsageAutoPolicy) {
			defer wg.Done()
			defer func() { <-sem }()
			if _, err := r.svc.runPolicy(ctx, p, TokenUsagePolicyRunTypeScheduled); err != nil {
				logger.LegacyPrintf("service.token_usage_policy_runner", "[TokenUsagePolicyRunner] policy=%d run failed: %v", p.ID, err)
			}
		}(policy)
	}
	wg.Wait()
}
