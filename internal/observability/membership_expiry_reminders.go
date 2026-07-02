package observability

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/email"
	"clientroll/internal/repository"

	"github.com/google/uuid"
)

// ExpiryReminderService sends membership expiry reminder emails to clients.
type ExpiryReminderService struct {
	db                *database.Postgres
	emailSvc          email.Provider
	emailTemplateRepo *repository.TenantEmailTemplateRepo
}

func NewExpiryReminderService(db *database.Postgres, emailSvc email.Provider, emailTemplateRepo *repository.TenantEmailTemplateRepo) *ExpiryReminderService {
	return &ExpiryReminderService{db: db, emailSvc: emailSvc, emailTemplateRepo: emailTemplateRepo}
}

// StartExpiryReminderScheduler runs daily and sends expiry reminders for memberships
// ending within 14 days that haven't been reminded yet.
func StartExpiryReminderScheduler(ctx context.Context, svc *ExpiryReminderService) {
	if svc == nil || svc.emailSvc == nil {
		log.Printf("expiry_reminder.scheduler skipped (missing dependencies)")
		return
	}
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	log.Printf("expiry_reminder.scheduler started")

	lastRunDate := ""
	for {
		select {
		case <-ctx.Done():
			log.Printf("expiry_reminder.scheduler stopped")
			return
		case <-ticker.C:
			today := time.Now().UTC().Format("2006-01-02")
			if today == lastRunDate {
				continue
			}
			svc.sendExpiryReminders(ctx)
			lastRunDate = today
		}
	}
}

type expiringMembership struct {
	MembershipID    uuid.UUID
	TenantID        uuid.UUID
	ClientID        uuid.UUID
	ClientName      string
	ClientEmail     string
	MembershipTitle string
	TenantName      string
	MinTermEndDate  time.Time
}

func (svc *ExpiryReminderService) sendExpiryReminders(ctx context.Context) {
	rows, err := svc.db.Pool.Query(ctx, `
		SELECT m.id, m.tenant_id, m.client_id,
		       c.name, c.email,
		       COALESCE(mt.title, 'Membership'),
		       COALESCE(t.name, ''),
		       m.min_term_end_date
		FROM memberships m
		JOIN clients c ON c.id = m.client_id AND c.tenant_id = m.tenant_id
		JOIN membership_types mt ON mt.id = m.membership_type_id AND mt.tenant_id = m.tenant_id
		JOIN tenants t ON t.id = m.tenant_id
		WHERE m.status = 'active'
		  AND m.continue_after_term = false
		  AND m.min_term_end_date BETWEEN NOW() AND NOW() + INTERVAL '14 days'
		  AND m.id NOT IN (
		      SELECT (metadata->>'membership_id')::uuid
		      FROM expiry_reminders_sent
		      WHERE sent_at > NOW() - INTERVAL '30 days'
		  )
		  AND c.email IS NOT NULL AND c.email <> ''
	`)
	if err != nil {
		log.Printf("expiry_reminder.query_failed err=%v", err)
		return
	}
	defer rows.Close()

	var memberships []expiringMembership
	for rows.Next() {
		var em expiringMembership
		if err := rows.Scan(&em.MembershipID, &em.TenantID, &em.ClientID,
			&em.ClientName, &em.ClientEmail, &em.MembershipTitle, &em.TenantName, &em.MinTermEndDate); err != nil {
			log.Printf("expiry_reminder.scan_failed err=%v", err)
			continue
		}
		memberships = append(memberships, em)
	}
	if err := rows.Err(); err != nil {
		log.Printf("expiry_reminder.rows_err err=%v", err)
		return
	}

	if len(memberships) == 0 {
		return
	}
	log.Printf("expiry_reminder.found %d memberships expiring soon", len(memberships))

	for _, em := range memberships {
		svc.sendReminder(ctx, em)
	}
}

func (svc *ExpiryReminderService) sendReminder(ctx context.Context, em expiringMembership) {
	templateKey := "membership_expiring"
	def, ok := email.OperationalTemplateByKey(templateKey)
	if !ok {
		return
	}
	enabled := def.DefaultEnabled
	subject := def.Subject
	body := def.Body
	if svc.emailTemplateRepo != nil {
		override, err := svc.emailTemplateRepo.GetByKey(ctx, em.TenantID, templateKey)
		if err != nil {
			log.Printf("expiry_reminder.template_lookup_failed tenant=%s err=%v", em.TenantID, err)
		} else if override != nil {
			enabled = override.Enabled
			if override.SubjectOverride != nil && strings.TrimSpace(*override.SubjectOverride) != "" {
				subject = *override.SubjectOverride
			}
			if override.BodyOverride != nil && strings.TrimSpace(*override.BodyOverride) != "" {
				body = *override.BodyOverride
			}
		}
	}
	if !enabled {
		return
	}

	vars := map[string]string{
		"client_name":      em.ClientName,
		"membership_title": em.MembershipTitle,
		"expiry_date":      em.MinTermEndDate.Format("2 January 2006"),
		"company_name":     em.TenantName,
	}
	render := func(input string) string {
		out := input
		for k, v := range vars {
			out = strings.ReplaceAll(out, "{{"+k+"}}", v)
		}
		return out
	}

	input := email.SendInput{
		To:      em.ClientEmail,
		Subject: render(subject),
		HTML:    email.WrapHTML(render(body)),
		Text:    render(subject),
	}
	if _, err := svc.emailSvc.Send(ctx, input); err != nil {
		log.Printf("expiry_reminder.send_failed tenant=%s membership=%s to=%s err=%v", em.TenantID, em.MembershipID, em.ClientEmail, err)
		return
	}
	log.Printf("expiry_reminder.sent tenant=%s membership=%s to=%s", em.TenantID, em.MembershipID, em.ClientEmail)

	svc.recordSent(ctx, em)
}

func (svc *ExpiryReminderService) recordSent(ctx context.Context, em expiringMembership) {
	_, err := svc.db.Pool.Exec(ctx, `
		INSERT INTO expiry_reminders_sent (tenant_id, membership_id, metadata, sent_at)
		VALUES ($1, $2, jsonb_build_object('membership_id', $2::text, 'client_id', $3::text), NOW())
	`, em.TenantID, em.MembershipID, em.ClientID)
	if err != nil {
		log.Printf("expiry_reminder.record_sent_failed tenant=%s membership=%s err=%v", em.TenantID, em.MembershipID, err)
	}
}

// EnsureExpiryRemindersTable creates the tracking table if it doesn't exist.
func EnsureExpiryRemindersTable(ctx context.Context, db *database.Postgres) {
	_, err := db.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS expiry_reminders_sent (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			tenant_id UUID NOT NULL,
			membership_id UUID NOT NULL,
			metadata JSONB,
			sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Printf("expiry_reminders_sent table creation failed: %v", err)
	} else {
		_, _ = db.Pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_expiry_reminders_membership ON expiry_reminders_sent (membership_id, sent_at)`)
	}
	// Also ensure the function used in the query exists — simpler to inline NOT IN subquery above
	_ = fmt.Sprintf("placeholder")
}
