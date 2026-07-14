package service

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"net/mail"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/yuin/goldmark"
)

const announcementEmailRecipientLimit = 10000

type AnnouncementEmailService struct {
	announcementRepo AnnouncementRepository
	userRepo         UserRepository
	broadcastRepo    AnnouncementEmailBroadcastRepository
	emailService     *EmailService
	settingService   *SettingService
}

func NewAnnouncementEmailService(
	announcementRepo AnnouncementRepository,
	userRepo UserRepository,
	broadcastRepo AnnouncementEmailBroadcastRepository,
	emailService *EmailService,
	settingService *SettingService,
) *AnnouncementEmailService {
	return &AnnouncementEmailService{
		announcementRepo: announcementRepo,
		userRepo:         userRepo,
		broadcastRepo:    broadcastRepo,
		emailService:     emailService,
		settingService:   settingService,
	}
}

func (s *AnnouncementEmailService) GetOverview(ctx context.Context, announcementID int64) (*AnnouncementEmailOverview, error) {
	announcement, err := s.announcementRepo.GetByID(ctx, announcementID)
	if err != nil {
		return nil, err
	}
	broadcast, err := s.broadcastRepo.GetByAnnouncementID(ctx, announcementID)
	if err == nil {
		return &AnnouncementEmailOverview{Broadcast: broadcast}, nil
	}
	if err != ErrAnnouncementEmailNotFound {
		return nil, err
	}
	canSend := announcement.IsActiveAt(time.Now())
	if _, err := s.emailService.GetSMTPConfig(ctx); err != nil {
		canSend = false
	}
	if !canSend {
		return &AnnouncementEmailOverview{CanSend: false}, nil
	}
	recipients, err := s.listRecipients(ctx, announcement)
	if err != nil {
		return nil, err
	}
	return &AnnouncementEmailOverview{EligibleCount: len(recipients), CanSend: len(recipients) > 0}, nil
}

func (s *AnnouncementEmailService) CreateBroadcast(ctx context.Context, announcementID, actorID int64) (*AnnouncementEmailBroadcast, error) {
	if _, err := s.emailService.GetSMTPConfig(ctx); err != nil {
		return nil, err
	}
	announcement, err := s.announcementRepo.GetByID(ctx, announcementID)
	if err != nil {
		return nil, err
	}
	if !announcement.IsActiveAt(time.Now()) {
		return nil, ErrAnnouncementEmailNotActive
	}
	if exists, err := s.broadcastRepo.ExistsByAnnouncementID(ctx, announcementID); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrAnnouncementEmailAlreadyExists
	}
	recipients, err := s.listRecipients(ctx, announcement)
	if err != nil {
		return nil, err
	}
	if len(recipients) == 0 {
		return nil, ErrAnnouncementEmailNoRecipients
	}
	siteName := "Sub2API"
	if s.settingService != nil {
		siteName = s.settingService.GetSiteName(ctx)
	}
	subject := fmt.Sprintf("[%s] %s", siteName, announcement.Title)
	bodyHTML, err := renderAnnouncementEmailHTML(siteName, announcement.Title, announcement.Content)
	if err != nil {
		return nil, err
	}
	return s.broadcastRepo.Create(ctx, announcementID, subject, bodyHTML, actorID, recipients)
}

func (s *AnnouncementEmailService) ListDeliveries(
	ctx context.Context,
	announcementID int64,
	params pagination.PaginationParams,
	status, search string,
) ([]AnnouncementEmailDelivery, *pagination.PaginationResult, error) {
	broadcast, err := s.broadcastRepo.GetByAnnouncementID(ctx, announcementID)
	if err != nil {
		return nil, nil, err
	}
	status = strings.TrimSpace(status)
	if status != "" && status != AnnouncementEmailDeliveryPending && status != AnnouncementEmailDeliveryProcessing && status != AnnouncementEmailDeliverySent && status != AnnouncementEmailDeliveryFailed {
		return nil, nil, ErrAnnouncementEmailNotFound
	}
	return s.broadcastRepo.ListDeliveries(ctx, broadcast.ID, params, status, strings.TrimSpace(search))
}

func (s *AnnouncementEmailService) RetryFailed(ctx context.Context, announcementID int64) (*AnnouncementEmailBroadcast, error) {
	broadcast, err := s.broadcastRepo.GetByAnnouncementID(ctx, announcementID)
	if err != nil {
		return nil, err
	}
	return s.broadcastRepo.RetryFailed(ctx, broadcast.ID)
}

func (s *AnnouncementEmailService) EnsureCanDelete(ctx context.Context, announcementID int64) error {
	exists, err := s.broadcastRepo.ExistsByAnnouncementID(ctx, announcementID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAnnouncementDeleteBroadcast
	}
	return nil
}

func (s *AnnouncementEmailService) listRecipients(ctx context.Context, announcement *Announcement) ([]AnnouncementEmailRecipient, error) {
	recipients := make([]AnnouncementEmailRecipient, 0)
	now := time.Now()
	for page := 1; ; page++ {
		users, result, err := s.userRepo.ListWithFilters(ctx, pagination.PaginationParams{
			Page: page, PageSize: 1000, SortBy: "id", SortOrder: "asc",
		}, UserListFilters{Status: StatusActive})
		if err != nil {
			return nil, fmt.Errorf("list announcement email recipients: %w", err)
		}
		for i := range users {
			u := &users[i]
			if !isDeliverableAnnouncementEmail(u.Email) {
				continue
			}
			activeGroups := make(map[int64]struct{}, len(u.Subscriptions))
			for j := range u.Subscriptions {
				if u.Subscriptions[j].ExpiresAt.After(now) {
					activeGroups[u.Subscriptions[j].GroupID] = struct{}{}
				}
			}
			if !announcement.Targeting.Matches(u.Balance, activeGroups) {
				continue
			}
			recipients = append(recipients, AnnouncementEmailRecipient{UserID: u.ID, Email: strings.TrimSpace(u.Email)})
			if len(recipients) > announcementEmailRecipientLimit {
				return nil, ErrAnnouncementEmailTooMany
			}
		}
		if result == nil || page >= result.Pages {
			break
		}
	}
	return recipients, nil
}

func isDeliverableAnnouncementEmail(value string) bool {
	email := strings.TrimSpace(value)
	if email == "" || isReservedEmail(email) {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && strings.EqualFold(address.Address, email)
}

func renderAnnouncementEmailHTML(siteName, title, markdown string) (string, error) {
	var content bytes.Buffer
	if err := goldmark.Convert([]byte(markdown), &content); err != nil {
		return "", fmt.Errorf("render announcement markdown: %w", err)
	}
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"></head><body style="margin:0;background:#f4f4f5;color:#18181b;font-family:Arial,sans-serif"><div style="max-width:680px;margin:0 auto;padding:32px 20px"><div style="background:#fff;border:1px solid #e4e4e7;padding:28px"><div style="font-size:13px;color:#71717a;margin-bottom:20px">%s</div><h1 style="font-size:24px;line-height:1.35;margin:0 0 24px">%s</h1><div style="font-size:15px;line-height:1.7">%s</div></div><div style="font-size:12px;color:#71717a;text-align:center;padding:18px">此邮件由 %s 发送，请勿直接回复。</div></div></body></html>`,
		html.EscapeString(siteName), html.EscapeString(title), content.String(), html.EscapeString(siteName)), nil
}
