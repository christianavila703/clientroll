package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"clientroll/internal/database"
	"clientroll/internal/models"
	"clientroll/internal/repository"

	"github.com/google/uuid"
)

// SaveAgreementSnapshotIfMissing persists agreement HTML for a membership when no snapshot exists yet.
// Used for staff-created memberships (public signup uses createAgreementSnapshot on SignupHandler).
func SaveAgreementSnapshotIfMissing(
	ctx context.Context,
	r *http.Request,
	tenant *models.Tenant,
	client *models.Client,
	membership *models.Membership,
	mt *models.MembershipType,
	termOption *models.TermOption,
	agreementRepo *repository.AgreementSnapshotRepo,
	settingsRepo *repository.SettingsRepo,
	termsRepo *repository.TermsRepo,
) {
	if agreementRepo == nil || settingsRepo == nil || termsRepo == nil {
		return
	}
	if tenant == nil || client == nil || membership == nil || mt == nil || termOption == nil {
		return
	}
	existing, err := agreementRepo.FindByMembership(ctx, membership.ID, tenant.ID)
	if err == nil && existing != nil {
		return
	}

	agreementSettings, _ := settingsRepo.GetAgreementSettings(ctx, tenant.ID)
	termsDocs, _ := termsRepo.GetForMembershipType(ctx, mt.ID, tenant.ID)

	snap := &repository.AgreementSnapshot{
		TenantID:                    tenant.ID,
		MembershipID:                membership.ID,
		ClientID:                    client.ID,
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

	ip := "staff-dashboard"
	ua := "clientroll/staff-membership-create"
	if r != nil {
		if v := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); v != "" {
			ip = v
		} else if r.RemoteAddr != "" {
			ip = r.RemoteAddr
		}
		if v := strings.TrimSpace(r.Header.Get("User-Agent")); v != "" {
			ua = v
		}
	}
	snap.IPAddress = &ip
	snap.UserAgent = &ua

	agreementHTML := RenderAgreementHTMLForSnapshot(snap, termsInputs, tenant)
	snap.AgreementHTML = &agreementHTML

	if _, err := agreementRepo.Create(ctx, snap, termsInputs); err != nil {
		log.Printf("agreement snapshot (staff) create failed membership=%s tenant=%s err=%v", membership.ID, tenant.ID, err)
	}
}

// EnsureSignupTenantDBContext sets Postgres RLS session tenant for public signup requests.
// Pool connections do not share session state; without this, RLS-protected reads can return
// no rows and agreement snapshots are skipped silently.
func EnsureSignupTenantDBContext(ctx context.Context, db *database.Postgres, tenantID uuid.UUID) error {
	if db == nil || tenantID == uuid.Nil {
		return nil
	}
	return db.SetTenantContext(ctx, tenantID.String())
}
