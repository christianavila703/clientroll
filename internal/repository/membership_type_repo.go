package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"clientroll/internal/database"
	"clientroll/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MembershipTypeRepo handles database operations for membership types.
type MembershipTypeRepo struct {
	db           *database.Postgres
	scheduleRepo *PaymentScheduleRepo
	termsRepo    *TermsRepo
}

func decodeMembershipEnabledPaymentFrequencies(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var freqs []string
	if err := json.Unmarshal(raw, &freqs); err != nil {
		return nil
	}
	return freqs
}

// NewMembershipTypeRepo creates a new membership type repository.
// scheduleRepo and termsRepo are optional — if set, term options returned
// from FindByID/FindBySlug/List include eager-loaded PaymentSchedules and
// AttachedTerms. When nil (e.g. in older call sites that haven't been wired
// yet) the term option is returned with the legacy columns only.
func NewMembershipTypeRepo(db *database.Postgres) *MembershipTypeRepo {
	return &MembershipTypeRepo{db: db}
}

// WithSchedules wires a PaymentScheduleRepo for eager loading.
func (r *MembershipTypeRepo) WithSchedules(s *PaymentScheduleRepo) *MembershipTypeRepo {
	r.scheduleRepo = s
	return r
}

// WithTerms wires a TermsRepo for per-term-option T&C eager loading.
func (r *MembershipTypeRepo) WithTerms(t *TermsRepo) *MembershipTypeRepo {
	r.termsRepo = t
	return r
}

// List returns all membership types for a tenant, with term options attached.
func (r *MembershipTypeRepo) List(ctx context.Context, tenantID uuid.UUID) ([]models.MembershipType, error) {
	query := `
		SELECT id, tenant_id, membership_name, membership_level, title, slug,
		       summary, description, inclusions, benefits,
		       top_image_url, currency_code, enabled_payment_frequencies, apply_sales_tax, is_active, is_most_popular, sort_order, use_custom_thankyou,
		       thankyou_heading, thankyou_body, thankyou_image_url, thankyou_redirect_url,
		       created_at, updated_at
		FROM membership_types WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY sort_order, created_at`

	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing membership types: %w", err)
	}
	defer rows.Close()
	types, err := r.scanMany(rows)
	if err != nil || len(types) == 0 {
		return types, err
	}

	allOpts, err := r.allTermOptionsForTenant(ctx, tenantID)
	if err != nil {
		return types, nil
	}
	for i := range types {
		types[i].TermOptions = allOpts[types[i].ID]
	}
	return types, nil
}

// allTermOptionsForTenant fetches every term option for a tenant in one query, keyed by membership_type_id.
func (r *MembershipTypeRepo) allTermOptionsForTenant(ctx context.Context, tenantID uuid.UUID) (map[uuid.UUID][]models.TermOption, error) {
	rows, err := r.db.Pool.Query(ctx, termOptionsListSQL+` WHERE tenant_id = $1 ORDER BY sort_order, duration_months`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[uuid.UUID][]models.TermOption)
	var ids []uuid.UUID
	for rows.Next() {
		to, err := scanTermOption(rows)
		if err != nil {
			return nil, err
		}
		m[to.MembershipTypeID] = append(m[to.MembershipTypeID], to)
		ids = append(ids, to.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	r.attachEnrichment(ctx, m, tenantID)
	return m, nil
}

// attachEnrichment loads payment schedules and per-term attached T&Cs in
// batch and stitches them into the term options in the provided map. Safe to
// call with a nil scheduleRepo / termsRepo (just returns without enrichment).
func (r *MembershipTypeRepo) attachEnrichment(ctx context.Context, byMT map[uuid.UUID][]models.TermOption, tenantID uuid.UUID) {
	if r.scheduleRepo != nil {
		if scheds, err := r.scheduleRepo.ListForTenant(ctx, tenantID); err == nil {
			for mt := range byMT {
				for i := range byMT[mt] {
					byMT[mt][i].PaymentSchedules = scheds[byMT[mt][i].ID]
				}
			}
		}
	}
	if r.termsRepo != nil {
		if attached, err := r.termsRepo.AttachedTermsForTenant(ctx, tenantID); err == nil {
			for mt := range byMT {
				for i := range byMT[mt] {
					byMT[mt][i].AttachedTerms = attached[byMT[mt][i].ID]
				}
			}
		}
	}
}

// FindByID returns a membership type with its term options.
func (r *MembershipTypeRepo) FindByID(ctx context.Context, id, tenantID uuid.UUID) (*models.MembershipType, error) {
	mt, err := r.findOne(ctx, "id = $1 AND tenant_id = $2", id, tenantID)
	if err != nil || mt == nil {
		return mt, err
	}
	mt.TermOptions, err = r.getTermOptions(ctx, mt.ID, tenantID)
	return mt, err
}

// FindBySlug returns a membership type by slug.
func (r *MembershipTypeRepo) FindBySlug(ctx context.Context, slug string, tenantID uuid.UUID) (*models.MembershipType, error) {
	mt, err := r.findOne(ctx, "slug = $1 AND tenant_id = $2", slug, tenantID)
	if err != nil || mt == nil {
		return mt, err
	}
	mt.TermOptions, err = r.getTermOptions(ctx, mt.ID, tenantID)
	return mt, err
}

// Create inserts a new membership type.
func (r *MembershipTypeRepo) Create(ctx context.Context, tenantID uuid.UUID, membershipName, membershipLevel *string, title, slug string, description, inclusions, topImageURL *string, sortOrder int, currencyCode string, applySalesTax bool) (*models.MembershipType, error) {
	query := `
		INSERT INTO membership_types (tenant_id, membership_name, membership_level, title, slug,
		    summary, description, inclusions, benefits,
		    top_image_url, currency_code, sort_order, apply_sales_tax)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, tenant_id, membership_name, membership_level, title, slug,
		          summary, description, inclusions, benefits,
		          top_image_url, currency_code, enabled_payment_frequencies, apply_sales_tax, is_active, is_most_popular, sort_order, use_custom_thankyou,
		          thankyou_heading, thankyou_body, thankyou_image_url, thankyou_redirect_url,
		          created_at, updated_at`

	var mt models.MembershipType
	var enabledPaymentFrequenciesRaw []byte
	err := r.db.Pool.QueryRow(ctx, query, tenantID, membershipName, membershipLevel, title, slug,
		nil, description, inclusions, nil,
		topImageURL, currencyCode, sortOrder, applySalesTax).Scan(r.scanFields(&mt, &enabledPaymentFrequenciesRaw)...)
	if err != nil {
		return nil, fmt.Errorf("creating membership type: %w", err)
	}
	mt.EnabledPaymentFrequencies = decodeMembershipEnabledPaymentFrequencies(enabledPaymentFrequenciesRaw)
	return &mt, nil
}

// Duplicate creates a deep copy of a membership type (inactive, with "(Copy)" suffix).
func (r *MembershipTypeRepo) Duplicate(ctx context.Context, id, tenantID uuid.UUID) (*models.MembershipType, error) {
	source, err := r.FindByID(ctx, id, tenantID)
	if err != nil || source == nil {
		return nil, fmt.Errorf("source membership type not found")
	}

	newSlug := source.Slug + "-copy"
	newTitle := source.Title + " (Copy)"

	mt, err := r.Create(ctx, tenantID, source.MembershipName, source.MembershipLevel, newTitle, newSlug,
		source.Description, source.Inclusions, source.TopImageURL,
		source.SortOrder+1, source.CurrencyCode, source.ApplySalesTax)
	if err != nil {
		return nil, err
	}

	// Copy term options
	for _, to := range source.TermOptions {
		_, err := r.CreateTermOption(ctx, tenantID, mt.ID, to.DurationMonths,
			to.FeeTotal, to.FeeWeekly, to.FeeFortnightly, to.FeeMonthly, to.FeeQuarterly, to.AllowUpfront, to.NumPayments,
			to.ContinueAfterTerm, to.CustomScheduleEnabled, to.CustomScheduleLabel, to.CustomScheduleAmount, to.CustomScheduleIntervalUnit, to.CustomScheduleIntervalCount, to.CustomScheduleNumPayments)
		if err != nil {
			return nil, fmt.Errorf("duplicating term option: %w", err)
		}
	}

	return r.FindByID(ctx, mt.ID, tenantID)
}

// CreateTermOption adds a term option to a membership type.
func (r *MembershipTypeRepo) CreateTermOption(
	ctx context.Context,
	tenantID, membershipTypeID uuid.UUID,
	durationMonths int,
	feeTotal float64,
	feeWeekly, feeFortnightly, feeMonthly, feeQuarterly *float64,
	allowUpfront bool,
	numPayments *int,
	continueAfterTerm bool,
	customScheduleEnabled bool,
	customScheduleLabel *string,
	customScheduleAmount *float64,
	customScheduleIntervalUnit *string,
	customScheduleIntervalCount *int,
	customScheduleNumPayments *int,
) (*models.TermOption, error) {
	query := `
		INSERT INTO membership_term_options (tenant_id, membership_type_id, duration_months,
		    duration_value, duration_unit,
		    fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront, num_payments,
		    continue_after_term,
		    custom_schedule_enabled, custom_schedule_label, custom_schedule_amount,
		    custom_schedule_interval_unit, custom_schedule_interval_count, custom_schedule_num_payments)
		VALUES ($1, $2, $3, $3, 'month', $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING ` + termOptionReturningCols

	var to models.TermOption
	err := r.db.Pool.QueryRow(ctx, query, tenantID, membershipTypeID, durationMonths,
		feeTotal, feeWeekly, feeFortnightly, feeMonthly, feeQuarterly, allowUpfront, numPayments,
		continueAfterTerm, customScheduleEnabled, customScheduleLabel, customScheduleAmount, customScheduleIntervalUnit, customScheduleIntervalCount, customScheduleNumPayments).Scan(
		termOptionScanArgs(&to)...)
	if err != nil {
		return nil, fmt.Errorf("creating term option: %w", err)
	}
	r.syncSchedulesFromLegacy(ctx, &to)
	return &to, nil
}

// syncSchedulesFromLegacy converts the legacy fee_* / custom_schedule_* values
// on the term option into payment_schedule rows and replaces the existing set.
// Called by Create/Update so the new schema stays in lock-step with the old.
// No-op if scheduleRepo isn't wired.
func (r *MembershipTypeRepo) syncSchedulesFromLegacy(ctx context.Context, to *models.TermOption) {
	if r.scheduleRepo == nil || to == nil {
		return
	}
	inputs := LegacyFeesToSchedules(
		to.DurationMonths, to.FeeTotal,
		to.FeeWeekly, to.FeeFortnightly, to.FeeMonthly, to.FeeQuarterly,
		to.AllowUpfront, to.ContinueAfterTerm,
		to.CustomScheduleEnabled, to.CustomScheduleLabel, to.CustomScheduleAmount,
		to.CustomScheduleIntervalUnit, to.CustomScheduleIntervalCount, to.CustomScheduleNumPayments,
	)
	if _, err := r.scheduleRepo.ReplaceForTermOption(ctx, to.TenantID, to.ID, inputs); err != nil {
		// Soak-window failure here means the new schedule rows are out of date,
		// but legacy code paths still work. Log loudly and keep going.
		fmt.Printf("WARN syncSchedulesFromLegacy term_option=%s tenant=%s err=%v\n",
			to.ID, to.TenantID, err)
	}
}

// UpdateTermOption updates a term option.
func (r *MembershipTypeRepo) UpdateTermOption(
	ctx context.Context,
	id, tenantID uuid.UUID,
	durationMonths int,
	feeTotal float64,
	feeWeekly, feeFortnightly, feeMonthly, feeQuarterly *float64,
	allowUpfront bool,
	numPayments *int,
	continueAfterTerm bool,
	customScheduleEnabled bool,
	customScheduleLabel *string,
	customScheduleAmount *float64,
	customScheduleIntervalUnit *string,
	customScheduleIntervalCount *int,
	customScheduleNumPayments *int,
) (*models.TermOption, error) {
	// Update both the legacy duration_months and the new duration_value so the
	// two representations stay in lock-step. duration_unit isn't touched here
	// — the new admin API drives it explicitly.
	query := `
		UPDATE membership_term_options SET duration_months=$3, duration_value=$3, fee_total=$4,
		    fee_weekly=$5, fee_fortnightly=$6, fee_monthly=$7, fee_quarterly=$8,
		    allow_upfront=$9, num_payments=$10, continue_after_term=$11, custom_schedule_enabled=$12,
		    custom_schedule_label=$13, custom_schedule_amount=$14,
		    custom_schedule_interval_unit=$15, custom_schedule_interval_count=$16,
		    custom_schedule_num_payments=$17, updated_at=NOW()
		WHERE id=$1 AND tenant_id=$2
		RETURNING ` + termOptionReturningCols

	var to models.TermOption
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID, durationMonths,
		feeTotal, feeWeekly, feeFortnightly, feeMonthly, feeQuarterly, allowUpfront, numPayments,
		continueAfterTerm, customScheduleEnabled, customScheduleLabel, customScheduleAmount, customScheduleIntervalUnit, customScheduleIntervalCount, customScheduleNumPayments).Scan(
		termOptionScanArgs(&to)...)
	if err != nil {
		return nil, fmt.Errorf("updating term option: %w", err)
	}
	r.syncSchedulesFromLegacy(ctx, &to)
	return &to, nil
}

// UpdateTermOptionMeta updates the v2-only metadata on a term option
// (duration_value, duration_unit, label, sort_order). Any nil pointer leaves
// the existing column unchanged. duration_value mirrors back into
// duration_months when set (so legacy code stays in sync during the soak).
func (r *MembershipTypeRepo) UpdateTermOptionMeta(
	ctx context.Context,
	id, tenantID uuid.UUID,
	durationValue *int,
	durationUnit *string,
	label *string,
	sortOrder *int,
) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE membership_term_options SET
			duration_value  = COALESCE($3, duration_value),
			duration_months = COALESCE($3, duration_months),
			duration_unit   = COALESCE($4, duration_unit),
			label           = CASE WHEN $5::text IS NOT NULL THEN $5 ELSE label END,
			sort_order      = COALESCE($6, sort_order),
			updated_at      = NOW()
		WHERE id = $1 AND tenant_id = $2`,
		id, tenantID, durationValue, durationUnit, label, sortOrder)
	return err
}

// SoftDelete marks a membership type as deleted. It disappears from listings but
// existing client memberships referencing it remain completely intact.
// A new type with the same name/slug can be created afterwards.
func (r *MembershipTypeRepo) SoftDelete(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE membership_types SET deleted_at = NOW(), is_active = false, is_most_popular = false, updated_at = NOW() WHERE id=$1 AND tenant_id=$2", id, tenantID)
	return err
}

// Archive sets is_active = false (soft delete). Existing memberships are unaffected.
func (r *MembershipTypeRepo) Archive(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE membership_types SET is_active = false, is_most_popular = false, updated_at = NOW() WHERE id=$1 AND tenant_id=$2", id, tenantID)
	return err
}

// Activate sets is_active = true.
func (r *MembershipTypeRepo) Activate(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE membership_types SET is_active = true, updated_at = NOW() WHERE id=$1 AND tenant_id=$2", id, tenantID)
	return err
}

// Delete hard-deletes a membership type ONLY if no memberships reference it.
func (r *MembershipTypeRepo) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	// Check for existing memberships
	var count int
	err := r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM memberships WHERE membership_type_id=$1 AND tenant_id=$2", id, tenantID).Scan(&count)
	if err != nil {
		return fmt.Errorf("checking membership references: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete: %d membership(s) reference this type — archive it instead", count)
	}
	// Delete term options first
	_, _ = r.db.Pool.Exec(ctx,
		"DELETE FROM membership_term_options WHERE membership_type_id=$1 AND tenant_id=$2", id, tenantID)
	// Delete the type
	_, err = r.db.Pool.Exec(ctx,
		"DELETE FROM membership_types WHERE id=$1 AND tenant_id=$2", id, tenantID)
	return err
}

// DeleteTermOption removes a term option.
func (r *MembershipTypeRepo) DeleteTermOption(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		"DELETE FROM membership_term_options WHERE id=$1 AND tenant_id=$2", id, tenantID)
	return err
}

// GetTermOption returns a single term option by ID, including its payment
// schedules and per-term attached T&Cs (when the repos are wired).
func (r *MembershipTypeRepo) GetTermOption(ctx context.Context, termOptionID, tenantID uuid.UUID) (*models.TermOption, error) {
	row := r.db.Pool.QueryRow(ctx,
		termOptionsListSQL+` WHERE id = $1 AND tenant_id = $2`,
		termOptionID, tenantID)
	to, err := scanTermOption(row)
	if err != nil {
		return nil, err
	}
	if r.scheduleRepo != nil {
		if scheds, err := r.scheduleRepo.ListForTermOption(ctx, to.ID, tenantID); err == nil {
			to.PaymentSchedules = scheds
		}
	}
	if r.termsRepo != nil {
		if attached, err := r.termsRepo.AttachedTermsForTermOption(ctx, to.ID, tenantID); err == nil {
			to.AttachedTerms = attached
		}
	}
	return &to, nil
}

func (r *MembershipTypeRepo) getTermOptions(ctx context.Context, membershipTypeID, tenantID uuid.UUID) ([]models.TermOption, error) {
	rows, err := r.db.Pool.Query(ctx,
		termOptionsListSQL+` WHERE membership_type_id = $1 AND tenant_id = $2 ORDER BY sort_order, duration_months`,
		membershipTypeID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var opts []models.TermOption
	for rows.Next() {
		to, err := scanTermOption(rows)
		if err != nil {
			return nil, err
		}
		opts = append(opts, to)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if r.scheduleRepo != nil || r.termsRepo != nil {
		byMT := map[uuid.UUID][]models.TermOption{membershipTypeID: opts}
		r.attachEnrichment(ctx, byMT, tenantID)
		opts = byMT[membershipTypeID]
	}
	return opts, nil
}

// Shared column list + scanner so the legacy and new-shape selects stay in sync.
const termOptionReturningCols = `id, tenant_id, membership_type_id, duration_months,
		duration_value, duration_unit, label, sort_order, archived_at,
		fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly,
		allow_upfront, num_payments, continue_after_term,
		custom_schedule_enabled, custom_schedule_label, custom_schedule_amount,
		custom_schedule_interval_unit, custom_schedule_interval_count, custom_schedule_num_payments`

const termOptionsListSQL = `SELECT ` + termOptionReturningCols + ` FROM membership_term_options`

// scanTermOption reads one row in the standard column order. Accepts either
// pgx.Row (single-row) or pgx.Rows (rows.Scan) via the interface.
type rowScanner interface{ Scan(dest ...any) error }

func scanTermOption(row rowScanner) (models.TermOption, error) {
	var to models.TermOption
	if err := row.Scan(termOptionScanArgs(&to)...); err != nil {
		return to, err
	}
	return to, nil
}

func termOptionScanArgs(to *models.TermOption) []any {
	return []any{
		&to.ID, &to.TenantID, &to.MembershipTypeID, &to.DurationMonths,
		&to.DurationValue, &to.DurationUnit, &to.Label, &to.SortOrder, &to.ArchivedAt,
		&to.FeeTotal, &to.FeeWeekly, &to.FeeFortnightly, &to.FeeMonthly, &to.FeeQuarterly,
		&to.AllowUpfront, &to.NumPayments, &to.ContinueAfterTerm,
		&to.CustomScheduleEnabled, &to.CustomScheduleLabel, &to.CustomScheduleAmount,
		&to.CustomScheduleIntervalUnit, &to.CustomScheduleIntervalCount, &to.CustomScheduleNumPayments,
	}
}

func (r *MembershipTypeRepo) findOne(ctx context.Context, where string, args ...interface{}) (*models.MembershipType, error) {
	query := `
		SELECT id, tenant_id, membership_name, membership_level, title, slug,
		       summary, description, inclusions, benefits,
		       top_image_url, currency_code, enabled_payment_frequencies, apply_sales_tax, is_active, is_most_popular, sort_order, use_custom_thankyou,
		       thankyou_heading, thankyou_body, thankyou_image_url, thankyou_redirect_url,
		       created_at, updated_at
		FROM membership_types WHERE ` + where

	var mt models.MembershipType
	var enabledPaymentFrequenciesRaw []byte
	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(r.scanFields(&mt, &enabledPaymentFrequenciesRaw)...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding membership type: %w", err)
	}
	mt.EnabledPaymentFrequencies = decodeMembershipEnabledPaymentFrequencies(enabledPaymentFrequenciesRaw)
	return &mt, nil
}

func (r *MembershipTypeRepo) scanFields(mt *models.MembershipType, enabledPaymentFrequenciesRaw *[]byte) []interface{} {
	return []interface{}{
		&mt.ID, &mt.TenantID, &mt.MembershipName, &mt.MembershipLevel, &mt.Title, &mt.Slug,
		&mt.Summary, &mt.Description, &mt.Inclusions, &mt.Benefits,
		&mt.TopImageURL, &mt.CurrencyCode, enabledPaymentFrequenciesRaw, &mt.ApplySalesTax, &mt.IsActive, &mt.IsMostPopular, &mt.SortOrder,
		&mt.UseCustomThankyou, &mt.ThankyouHeading, &mt.ThankyouBody,
		&mt.ThankyouImageURL, &mt.ThankyouRedirectURL, &mt.CreatedAt, &mt.UpdatedAt,
	}
}

// SetMostPopular marks one membership type as most popular for a tenant.
func (r *MembershipTypeRepo) SetMostPopular(ctx context.Context, tenantID, id uuid.UUID) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin set most popular: %w", err)
	}
	defer tx.Rollback(ctx)

	var groupKey string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(membership_name, ''), title)
		FROM membership_types
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, tenantID, id).Scan(&groupKey); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("membership type not found")
		}
		return fmt.Errorf("load most popular group: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE membership_types
		SET is_most_popular = false
		WHERE tenant_id = $1
		  AND deleted_at IS NULL
		  AND COALESCE(NULLIF(membership_name, ''), title) = $2
	`, tenantID, groupKey); err != nil {
		return fmt.Errorf("clear most popular flags for group: %w", err)
	}
	if _, err := tx.Exec(ctx, "UPDATE membership_types SET is_most_popular = true WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL", tenantID, id); err != nil {
		return fmt.Errorf("set most popular flag: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit set most popular: %w", err)
	}
	return nil
}

// ClearMostPopular removes the most-popular flag from a membership type.
func (r *MembershipTypeRepo) ClearMostPopular(ctx context.Context, tenantID, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE membership_types SET is_most_popular = false, updated_at = NOW() WHERE tenant_id = $1 AND id = $2", tenantID, id)
	if err != nil {
		return fmt.Errorf("clear most popular: %w", err)
	}
	return nil
}

// UpdateBasics updates editable core fields for a membership type.
func (r *MembershipTypeRepo) UpdateBasics(
	ctx context.Context,
	id, tenantID uuid.UUID,
	membershipName, membershipLevel, title, slug, summary, description, inclusions, benefits, topImageURL *string,
	currencyCode *string,
	sortOrder *int,
	enabledPaymentFrequencies *[]string,
	applySalesTax *bool,
) (*models.MembershipType, error) {
	query := `
		UPDATE membership_types SET
			membership_name = COALESCE($3, membership_name),
			membership_level = COALESCE($4, membership_level),
			title = COALESCE($5, title),
			slug = COALESCE($6, slug),
			summary = COALESCE($7, summary),
			description = COALESCE($8, description),
			inclusions = COALESCE($9, inclusions),
			benefits = COALESCE($10, benefits),
			top_image_url = COALESCE($11, top_image_url),
			currency_code = COALESCE($12, currency_code),
			sort_order = COALESCE($13, sort_order),
			enabled_payment_frequencies = CASE
				WHEN $14::jsonb IS NULL THEN enabled_payment_frequencies
				ELSE $14::jsonb
			END,
			apply_sales_tax = COALESCE($15, apply_sales_tax),
			updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, membership_name, membership_level, title, slug,
		          summary, description, inclusions, benefits,
		          top_image_url, currency_code, enabled_payment_frequencies, apply_sales_tax, is_active, is_most_popular, sort_order, use_custom_thankyou,
		          thankyou_heading, thankyou_body, thankyou_image_url, thankyou_redirect_url,
		          created_at, updated_at`

	var mt models.MembershipType
	var enabledPaymentFrequenciesRaw []byte
	var enabledPaymentFrequenciesJSON []byte
	if enabledPaymentFrequencies != nil {
		encoded, err := json.Marshal(*enabledPaymentFrequencies)
		if err != nil {
			return nil, fmt.Errorf("marshalling enabled payment frequencies: %w", err)
		}
		enabledPaymentFrequenciesJSON = encoded
	}
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID, membershipName, membershipLevel, title, slug, summary, description, inclusions, benefits, topImageURL, currencyCode, sortOrder, enabledPaymentFrequenciesJSON, applySalesTax).
		Scan(r.scanFields(&mt, &enabledPaymentFrequenciesRaw)...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("updating membership type basics: %w", err)
	}
	mt.EnabledPaymentFrequencies = decodeMembershipEnabledPaymentFrequencies(enabledPaymentFrequenciesRaw)
	return &mt, nil
}

func (r *MembershipTypeRepo) scanMany(rows pgx.Rows) ([]models.MembershipType, error) {
	var items []models.MembershipType
	for rows.Next() {
		var mt models.MembershipType
		var enabledPaymentFrequenciesRaw []byte
		if err := rows.Scan(r.scanFields(&mt, &enabledPaymentFrequenciesRaw)...); err != nil {
			return nil, fmt.Errorf("scanning membership type: %w", err)
		}
		mt.EnabledPaymentFrequencies = decodeMembershipEnabledPaymentFrequencies(enabledPaymentFrequenciesRaw)
		items = append(items, mt)
	}
	return items, rows.Err()
}

// GenerateSlug creates a URL-safe slug from a title.
func GenerateSlug(title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove non-alphanumeric except hyphens
	var result []byte
	for _, c := range []byte(slug) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result = append(result, c)
		}
	}
	return string(result)
}
