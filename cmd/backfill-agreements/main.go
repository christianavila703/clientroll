package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"clientroll/internal/database"
	"clientroll/internal/handlers"
	"clientroll/internal/repository"

	"github.com/google/uuid"
)

type missingMembershipRow struct {
	MembershipID     uuid.UUID
	TenantID         uuid.UUID
	ClientID         uuid.UUID
	MembershipTypeID uuid.UUID
	TermOptionID     uuid.UUID
}

func main() {
	var (
		tenantFilter string
		limit        int
		dryRun       bool
	)
	flag.StringVar(&tenantFilter, "tenant", "", "optional tenant UUID filter")
	flag.IntVar(&limit, "limit", 0, "optional max number of memberships to process")
	flag.BoolVar(&dryRun, "dry-run", false, "preview rows without writing snapshots")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(dbURL) == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	pg, err := database.NewPostgres(dbURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pg.Close()

	ctx := context.Background()

	rows, err := listMissingMemberships(ctx, pg, tenantFilter, limit)
	if err != nil {
		log.Fatalf("list memberships missing agreement snapshots: %v", err)
	}
	if len(rows) == 0 {
		log.Println("No memberships require backfill.")
		return
	}

	agreementRepo := repository.NewAgreementSnapshotRepo(pg)
	tenantRepo := repository.NewTenantRepo(pg)
	clientRepo := repository.NewClientRepo(pg)
	memberRepo := repository.NewMembershipRepo(pg)
	mtRepo := repository.NewMembershipTypeRepo(pg)
	termsRepo := repository.NewTermsRepo(pg)
	settingsRepo := repository.NewSettingsRepo(pg)

	created := 0
	skipped := 0
	failed := 0

	for _, row := range rows {
		if dryRun {
			log.Printf("[dry-run] would backfill membership=%s tenant=%s", row.MembershipID, row.TenantID)
			continue
		}

		if err := backfillOne(ctx, row, agreementRepo, tenantRepo, clientRepo, memberRepo, mtRepo, termsRepo, settingsRepo); err != nil {
			failed++
			log.Printf("backfill failed membership=%s tenant=%s err=%v", row.MembershipID, row.TenantID, err)
			continue
		}
		created++
	}

	if dryRun {
		log.Printf("Dry run complete. matched=%d", len(rows))
		return
	}

	// Recount remaining to report skipped/non-eligible alongside failures.
	remaining, _ := listMissingMemberships(ctx, pg, tenantFilter, limit)
	if len(remaining) > 0 {
		skipped = len(remaining)
	}

	log.Printf("Backfill complete. created=%d failed=%d remaining_without_snapshot=%d", created, failed, skipped)
}

func listMissingMemberships(ctx context.Context, pg *database.Postgres, tenantFilter string, limit int) ([]missingMembershipRow, error) {
	query := `
		SELECT m.id, m.tenant_id, m.client_id, m.membership_type_id, m.term_option_id
		FROM memberships m
		LEFT JOIN membership_agreement_snapshots mas
			ON mas.membership_id = m.id AND mas.tenant_id = m.tenant_id
		WHERE mas.id IS NULL
	`
	args := make([]interface{}, 0, 2)
	argN := 1

	if strings.TrimSpace(tenantFilter) != "" {
		tid, err := uuid.Parse(strings.TrimSpace(tenantFilter))
		if err != nil {
			return nil, fmt.Errorf("invalid tenant uuid: %w", err)
		}
		query += fmt.Sprintf(" AND m.tenant_id = $%d", argN)
		args = append(args, tid)
		argN++
	}

	query += " ORDER BY m.created_at ASC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argN)
		args = append(args, limit)
	}

	rows, err := pg.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]missingMembershipRow, 0)
	for rows.Next() {
		var r missingMembershipRow
		if err := rows.Scan(&r.MembershipID, &r.TenantID, &r.ClientID, &r.MembershipTypeID, &r.TermOptionID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func backfillOne(
	ctx context.Context,
	row missingMembershipRow,
	agreementRepo *repository.AgreementSnapshotRepo,
	tenantRepo *repository.TenantRepo,
	clientRepo *repository.ClientRepo,
	memberRepo *repository.MembershipRepo,
	mtRepo *repository.MembershipTypeRepo,
	termsRepo *repository.TermsRepo,
	settingsRepo *repository.SettingsRepo,
) error {
	tenant, err := tenantRepo.FindByID(ctx, row.TenantID)
	if err != nil || tenant == nil {
		return fmt.Errorf("tenant lookup: %w", err)
	}
	client, err := clientRepo.FindByID(ctx, row.ClientID, row.TenantID)
	if err != nil || client == nil {
		return fmt.Errorf("client lookup: %w", err)
	}
	membership, err := memberRepo.FindByID(ctx, row.MembershipID, row.TenantID)
	if err != nil || membership == nil {
		return fmt.Errorf("membership lookup: %w", err)
	}
	mt, err := mtRepo.FindByID(ctx, row.MembershipTypeID, row.TenantID)
	if err != nil || mt == nil {
		return fmt.Errorf("membership type lookup: %w", err)
	}
	termOption, err := mtRepo.GetTermOption(ctx, row.TermOptionID, row.TenantID)
	if err != nil || termOption == nil {
		return fmt.Errorf("term option lookup: %w", err)
	}
	agreementSettings, _ := settingsRepo.GetAgreementSettings(ctx, row.TenantID)
	termsDocs, _ := termsRepo.GetForMembershipType(ctx, row.MembershipTypeID, row.TenantID)

	snap := &repository.AgreementSnapshot{
		TenantID:                    row.TenantID,
		MembershipID:                row.MembershipID,
		ClientID:                    row.ClientID,
		ClientName:                  client.Name,
		ClientEmail:                 client.Email,
		ClientPhone:                 client.Phone,
		ClientGender:                client.Gender,
		ClientEmergencyContactName:  client.EmergencyContactName,
		ClientEmergencyContactPhone: client.EmergencyContactPhone,
		MembershipTypeTitle:         mt.Title,
		MembershipName:              mt.MembershipName,
		MembershipLevel:             mt.MembershipLevel,
		MembershipDescription:       mt.Description,
		MembershipInclusions:        mt.Inclusions,
		MembershipBenefits:          mt.Benefits,
		TermMonths:                  termOption.DurationMonths,
		TotalFee:                    termOption.FeeTotal,
		PaymentMethod:               membership.PaymentMethod,
		PaymentFrequency:            membership.PaymentFrequency,
		PaymentAmount:               membership.PaymentAmount,
		PaymentIntervalUnit:         membership.PaymentIntervalUnit,
		PaymentIntervalCount:        membership.PaymentIntervalCount,
		NumPayments:                 membership.NumPayments,
		StartDate:                   membership.StartDate.Format("2006-01-02"),
		FeeWeekly:                   termOption.FeeWeekly,
		FeeFortnightly:              termOption.FeeFortnightly,
		FeeMonthly:                  termOption.FeeMonthly,
		FeeQuarterly:                termOption.FeeQuarterly,
		AllowUpfront:                termOption.AllowUpfront,
		CustomScheduleLabel:         termOption.CustomScheduleLabel,
		CustomScheduleAmount:        termOption.CustomScheduleAmount,
		CustomScheduleIntervalUnit:  termOption.CustomScheduleIntervalUnit,
		CustomScheduleIntervalCount: termOption.CustomScheduleIntervalCount,
		CustomScheduleNumPayments:   termOption.CustomScheduleNumPayments,
		MemberSignatureData:         membership.MemberSignature,
		MemberSignedAt:              membership.MemberSignedAt,
		StaffSignatureData:          membership.StaffSignature,
		StaffSignedAt:               membership.StaffSignedAt,
	}

	if !membership.MinTermEndDate.IsZero() {
		endDate := membership.MinTermEndDate.Format("2006-01-02")
		snap.EndDate = &endDate
	}
	termLabel := fmt.Sprintf("%d months", termOption.DurationMonths)
	snap.TermLabel = &termLabel

	if client.BirthDate != nil {
		dob := client.BirthDate.Format("2006-01-02")
		snap.ClientDateOfBirth = &dob
	}
	if client.Address != nil {
		addr := strings.TrimSpace(*client.Address)
		if client.UnitNumber != nil && strings.TrimSpace(*client.UnitNumber) != "" {
			addr = "Unit " + strings.TrimSpace(*client.UnitNumber) + ", " + addr
		}
		if client.Suburb != nil && strings.TrimSpace(*client.Suburb) != "" {
			addr += ", " + strings.TrimSpace(*client.Suburb)
		}
		if client.State != nil && strings.TrimSpace(*client.State) != "" {
			addr += " " + strings.TrimSpace(*client.State)
		}
		if client.Postcode != nil && strings.TrimSpace(*client.Postcode) != "" {
			addr += " " + strings.TrimSpace(*client.Postcode)
		}
		addr = strings.TrimSpace(addr)
		if addr != "" {
			snap.ClientAddress = &addr
		}
	}

	if agreementSettings != nil {
		snap.OrgCompanyName = agreementSettings.CompanyName
		snap.OrgTradingName = agreementSettings.TradingName
		snap.OrgABN = agreementSettings.ABN
		snap.OrgACN = agreementSettings.ACN
		snap.OrgEmail = agreementSettings.Email
		snap.OrgPhone = agreementSettings.Phone
		orgAddr := ""
		if agreementSettings.Address != nil {
			orgAddr = strings.TrimSpace(*agreementSettings.Address)
		}
		if agreementSettings.State != nil && strings.TrimSpace(*agreementSettings.State) != "" {
			orgAddr = strings.TrimSpace(orgAddr + " " + strings.TrimSpace(*agreementSettings.State))
		}
		if agreementSettings.Postcode != nil && strings.TrimSpace(*agreementSettings.Postcode) != "" {
			orgAddr = strings.TrimSpace(orgAddr + " " + strings.TrimSpace(*agreementSettings.Postcode))
		}
		if orgAddr != "" {
			snap.OrgAddress = &orgAddr
		}
	}

	termsInputs := make([]repository.AcceptedTermInput, 0, len(termsDocs))
	for _, t := range termsDocs {
		termsInputs = append(termsInputs, repository.AcceptedTermInput{
			TermsID:     t.ID,
			Title:       t.Title,
			Version:     t.Version,
			ContentHTML: t.Content,
		})
	}

	agreementHTML := handlers.RenderAgreementHTMLForSnapshot(snap, termsInputs, tenant)
	snap.AgreementHTML = &agreementHTML

	backfillMarker := fmt.Sprintf("backfill-%s", time.Now().UTC().Format(time.RFC3339))
	snap.IPAddress = &backfillMarker
	snap.UserAgent = &backfillMarker

	_, err = agreementRepo.Create(ctx, snap, termsInputs)
	return err
}
