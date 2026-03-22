package repository

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

type AgreementSnapshot struct {
	ID           uuid.UUID `json:"id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	MembershipID uuid.UUID `json:"membership_id"`
	ClientID     uuid.UUID `json:"client_id"`

	// Client details
	ClientName                  string  `json:"client_name"`
	ClientEmail                 *string `json:"client_email"`
	ClientPhone                 *string `json:"client_phone"`
	ClientAddress               *string `json:"client_address"`
	ClientDateOfBirth           *string `json:"client_date_of_birth"`
	ClientGender                *string `json:"client_gender"`
	ClientEmergencyContactName  *string `json:"client_emergency_contact_name"`
	ClientEmergencyContactPhone *string `json:"client_emergency_contact_phone"`

	// Membership details
	MembershipTypeTitle  string  `json:"membership_type_title"`
	MembershipName       *string `json:"membership_name"`
	MembershipLevel      *string `json:"membership_level"`
	MembershipDescription *string `json:"membership_description"`
	MembershipInclusions *string `json:"membership_inclusions"`
	MembershipBenefits   *string `json:"membership_benefits"`

	// Term & payment details
	TermMonths          int      `json:"term_months"`
	TermLabel           *string  `json:"term_label"`
	TotalFee            float64  `json:"total_fee"`
	PaymentMethod       *string  `json:"payment_method"`
	PaymentFrequency    *string  `json:"payment_frequency"`
	PaymentAmount       *float64 `json:"payment_amount"`
	PaymentIntervalUnit *string  `json:"payment_interval_unit"`
	PaymentIntervalCount int     `json:"payment_interval_count"`
	NumPayments         *int     `json:"num_payments"`
	StartDate           string   `json:"start_date"`
	EndDate             *string  `json:"end_date"`

	// Fee breakdown
	FeeWeekly      *float64 `json:"fee_weekly"`
	FeeFortnightly *float64 `json:"fee_fortnightly"`
	FeeMonthly     *float64 `json:"fee_monthly"`
	FeeQuarterly   *float64 `json:"fee_quarterly"`
	AllowUpfront   bool     `json:"allow_upfront"`

	// Custom schedule
	CustomScheduleLabel         *string  `json:"custom_schedule_label"`
	CustomScheduleAmount        *float64 `json:"custom_schedule_amount"`
	CustomScheduleIntervalUnit  *string  `json:"custom_schedule_interval_unit"`
	CustomScheduleIntervalCount *int     `json:"custom_schedule_interval_count"`
	CustomScheduleNumPayments   *int     `json:"custom_schedule_num_payments"`

	// Organisation details
	OrgCompanyName  *string `json:"org_company_name"`
	OrgTradingName  *string `json:"org_trading_name"`
	OrgABN          *string `json:"org_abn"`
	OrgACN          *string `json:"org_acn"`
	OrgAddress      *string `json:"org_address"`
	OrgEmail        *string `json:"org_email"`
	OrgPhone        *string `json:"org_phone"`

	// Signatures
	MemberSignatureData *string    `json:"member_signature_data"`
	MemberSignedAt      *time.Time `json:"member_signed_at"`
	StaffSignatureData  *string    `json:"staff_signature_data"`
	StaffSignedAt       *time.Time `json:"staff_signed_at"`

	AcceptedTerms json.RawMessage `json:"accepted_terms"`
	AgreementHTML *string         `json:"agreement_html"`

	IPAddress *string   `json:"ip_address"`
	UserAgent *string   `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}

type AcceptedTermRecord struct {
	TermsID     string `json:"terms_id"`
	Title       string `json:"title"`
	Version     int    `json:"version"`
	ContentHash string `json:"content_hash"`
	AcceptedAt  string `json:"accepted_at"`
}

type AcceptedTermInput struct {
	TermsID     uuid.UUID
	Title       string
	Version     int
	ContentHTML string
}

type AgreementSnapshotRepo struct {
	db *database.Postgres
}

func NewAgreementSnapshotRepo(db *database.Postgres) *AgreementSnapshotRepo {
	return &AgreementSnapshotRepo{db: db}
}

func (r *AgreementSnapshotRepo) Create(ctx context.Context, snap *AgreementSnapshot, termsInputs []AcceptedTermInput) (*AgreementSnapshot, error) {
	now := time.Now()

	termsJSON := make([]AcceptedTermRecord, 0, len(termsInputs))
	for _, t := range termsInputs {
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(t.ContentHTML)))
		termsJSON = append(termsJSON, AcceptedTermRecord{
			TermsID:     t.TermsID.String(),
			Title:       t.Title,
			Version:     t.Version,
			ContentHash: hash,
			AcceptedAt:  now.Format(time.RFC3339),
		})
	}
	termsBytes, _ := json.Marshal(termsJSON)
	snap.AcceptedTerms = termsBytes

	query := `
		INSERT INTO membership_agreement_snapshots (
			tenant_id, membership_id, client_id,
			client_name, client_email, client_phone, client_address,
			client_date_of_birth, client_gender, client_emergency_contact_name, client_emergency_contact_phone,
			membership_type_title, membership_name, membership_level,
			membership_description, membership_inclusions, membership_benefits,
			term_months, term_label, total_fee,
			payment_method, payment_frequency, payment_amount,
			payment_interval_unit, payment_interval_count, num_payments,
			start_date, end_date,
			fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront,
			custom_schedule_label, custom_schedule_amount,
			custom_schedule_interval_unit, custom_schedule_interval_count, custom_schedule_num_payments,
			org_company_name, org_trading_name, org_abn, org_acn, org_address, org_email, org_phone,
			member_signature_data, member_signed_at, staff_signature_data, staff_signed_at,
			accepted_terms, agreement_html, ip_address, user_agent
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
			$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
			$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,
			$31,$32,$33,$34,$35,$36,$37,$38,$39,$40,
			$41,$42,$43,$44,$45,$46,$47,$48,$49,$50,$51,$52,$53
		) RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query,
		snap.TenantID, snap.MembershipID, snap.ClientID,
		snap.ClientName, snap.ClientEmail, snap.ClientPhone, snap.ClientAddress,
		snap.ClientDateOfBirth, snap.ClientGender, snap.ClientEmergencyContactName, snap.ClientEmergencyContactPhone,
		snap.MembershipTypeTitle, snap.MembershipName, snap.MembershipLevel,
		snap.MembershipDescription, snap.MembershipInclusions, snap.MembershipBenefits,
		snap.TermMonths, snap.TermLabel, snap.TotalFee,
		snap.PaymentMethod, snap.PaymentFrequency, snap.PaymentAmount,
		snap.PaymentIntervalUnit, snap.PaymentIntervalCount, snap.NumPayments,
		snap.StartDate, snap.EndDate,
		snap.FeeWeekly, snap.FeeFortnightly, snap.FeeMonthly, snap.FeeQuarterly, snap.AllowUpfront,
		snap.CustomScheduleLabel, snap.CustomScheduleAmount,
		snap.CustomScheduleIntervalUnit, snap.CustomScheduleIntervalCount, snap.CustomScheduleNumPayments,
		snap.OrgCompanyName, snap.OrgTradingName, snap.OrgABN, snap.OrgACN, snap.OrgAddress, snap.OrgEmail, snap.OrgPhone,
		snap.MemberSignatureData, snap.MemberSignedAt, snap.StaffSignatureData, snap.StaffSignedAt,
		termsBytes, snap.AgreementHTML, snap.IPAddress, snap.UserAgent,
	).Scan(&snap.ID, &snap.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating agreement snapshot: %w", err)
	}

	for _, t := range termsInputs {
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(t.ContentHTML)))
		_, err := r.db.Pool.Exec(ctx, `
			INSERT INTO membership_accepted_terms (tenant_id, membership_id, terms_id, snapshot_id, terms_title, terms_version, terms_content_html, content_sha256)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (membership_id, terms_id) DO NOTHING`,
			snap.TenantID, snap.MembershipID, t.TermsID, snap.ID,
			t.Title, t.Version, t.ContentHTML, hash,
		)
		if err != nil {
			return nil, fmt.Errorf("storing accepted term %s: %w", t.TermsID, err)
		}
	}

	return snap, nil
}

func (r *AgreementSnapshotRepo) FindByMembership(ctx context.Context, membershipID, tenantID uuid.UUID) (*AgreementSnapshot, error) {
	query := `
		SELECT id, tenant_id, membership_id, client_id,
			client_name, client_email, client_phone, client_address,
			client_date_of_birth::text, client_gender, client_emergency_contact_name, client_emergency_contact_phone,
			membership_type_title, membership_name, membership_level,
			membership_description, membership_inclusions, membership_benefits,
			term_months, term_label, total_fee,
			payment_method, payment_frequency, payment_amount,
			payment_interval_unit, payment_interval_count, num_payments,
			start_date::text, end_date::text,
			fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront,
			custom_schedule_label, custom_schedule_amount,
			custom_schedule_interval_unit, custom_schedule_interval_count, custom_schedule_num_payments,
			org_company_name, org_trading_name, org_abn, org_acn, org_address, org_email, org_phone,
			member_signature_data, member_signed_at, staff_signature_data, staff_signed_at,
			accepted_terms, agreement_html, ip_address, user_agent, created_at
		FROM membership_agreement_snapshots
		WHERE membership_id = $1 AND tenant_id = $2
		ORDER BY created_at DESC LIMIT 1`

	var s AgreementSnapshot
	err := r.db.Pool.QueryRow(ctx, query, membershipID, tenantID).Scan(
		&s.ID, &s.TenantID, &s.MembershipID, &s.ClientID,
		&s.ClientName, &s.ClientEmail, &s.ClientPhone, &s.ClientAddress,
		&s.ClientDateOfBirth, &s.ClientGender, &s.ClientEmergencyContactName, &s.ClientEmergencyContactPhone,
		&s.MembershipTypeTitle, &s.MembershipName, &s.MembershipLevel,
		&s.MembershipDescription, &s.MembershipInclusions, &s.MembershipBenefits,
		&s.TermMonths, &s.TermLabel, &s.TotalFee,
		&s.PaymentMethod, &s.PaymentFrequency, &s.PaymentAmount,
		&s.PaymentIntervalUnit, &s.PaymentIntervalCount, &s.NumPayments,
		&s.StartDate, &s.EndDate,
		&s.FeeWeekly, &s.FeeFortnightly, &s.FeeMonthly, &s.FeeQuarterly, &s.AllowUpfront,
		&s.CustomScheduleLabel, &s.CustomScheduleAmount,
		&s.CustomScheduleIntervalUnit, &s.CustomScheduleIntervalCount, &s.CustomScheduleNumPayments,
		&s.OrgCompanyName, &s.OrgTradingName, &s.OrgABN, &s.OrgACN, &s.OrgAddress, &s.OrgEmail, &s.OrgPhone,
		&s.MemberSignatureData, &s.MemberSignedAt, &s.StaffSignatureData, &s.StaffSignedAt,
		&s.AcceptedTerms, &s.AgreementHTML, &s.IPAddress, &s.UserAgent, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

type AcceptedTermRow struct {
	Title       string    `json:"title"`
	Version     int       `json:"version"`
	ContentHTML string    `json:"content_html"`
	AcceptedAt  time.Time `json:"accepted_at"`
}

func (r *AgreementSnapshotRepo) GetAcceptedTerms(ctx context.Context, membershipID, tenantID uuid.UUID) ([]AcceptedTermRow, error) {
	query := `
		SELECT terms_title, terms_version, terms_content_html, accepted_at
		FROM membership_accepted_terms
		WHERE membership_id = $1 AND tenant_id = $2
		ORDER BY accepted_at`

	rows, err := r.db.Pool.Query(ctx, query, membershipID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []AcceptedTermRow
	for rows.Next() {
		var t AcceptedTermRow
		if err := rows.Scan(&t.Title, &t.Version, &t.ContentHTML, &t.AcceptedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}
