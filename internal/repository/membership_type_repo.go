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
	db *database.Postgres
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
func NewMembershipTypeRepo(db *database.Postgres) *MembershipTypeRepo {
	return &MembershipTypeRepo{db: db}
}

// List returns all membership types for a tenant, with term options attached.
func (r *MembershipTypeRepo) List(ctx context.Context, tenantID uuid.UUID) ([]models.MembershipType, error) {
	query := `
		SELECT id, tenant_id, membership_name, membership_level, title, slug,
		       summary, description, inclusions, benefits,
		       top_image_url, currency_code, enabled_payment_frequencies, is_active, is_most_popular, sort_order, use_custom_thankyou,
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
	query := `
		SELECT id, tenant_id, membership_type_id, duration_months,
		       fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront, num_payments,
		       custom_schedule_enabled, custom_schedule_label, custom_schedule_amount,
		       custom_schedule_interval_unit, custom_schedule_interval_count, custom_schedule_num_payments
		FROM membership_term_options
		WHERE tenant_id = $1
		ORDER BY duration_months`
	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[uuid.UUID][]models.TermOption)
	for rows.Next() {
		var to models.TermOption
		if err := rows.Scan(&to.ID, &to.TenantID, &to.MembershipTypeID,
			&to.DurationMonths, &to.FeeTotal, &to.FeeWeekly,
			&to.FeeFortnightly, &to.FeeMonthly, &to.FeeQuarterly, &to.AllowUpfront, &to.NumPayments,
			&to.CustomScheduleEnabled, &to.CustomScheduleLabel, &to.CustomScheduleAmount,
			&to.CustomScheduleIntervalUnit, &to.CustomScheduleIntervalCount, &to.CustomScheduleNumPayments); err != nil {
			return nil, err
		}
		m[to.MembershipTypeID] = append(m[to.MembershipTypeID], to)
	}
	return m, rows.Err()
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
func (r *MembershipTypeRepo) Create(ctx context.Context, tenantID uuid.UUID, membershipName, membershipLevel *string, title, slug string, description, inclusions, topImageURL *string, sortOrder int, currencyCode string) (*models.MembershipType, error) {
	query := `
		INSERT INTO membership_types (tenant_id, membership_name, membership_level, title, slug,
		    summary, description, inclusions, benefits,
		    top_image_url, currency_code, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, tenant_id, membership_name, membership_level, title, slug,
		          summary, description, inclusions, benefits,
		          top_image_url, currency_code, enabled_payment_frequencies, is_active, is_most_popular, sort_order, use_custom_thankyou,
		          thankyou_heading, thankyou_body, thankyou_image_url, thankyou_redirect_url,
		          created_at, updated_at`

	var mt models.MembershipType
	var enabledPaymentFrequenciesRaw []byte
	err := r.db.Pool.QueryRow(ctx, query, tenantID, membershipName, membershipLevel, title, slug,
		nil, description, inclusions, nil,
		topImageURL, currencyCode, sortOrder).Scan(r.scanFields(&mt, &enabledPaymentFrequenciesRaw)...)
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
		source.SortOrder+1, source.CurrencyCode)
	if err != nil {
		return nil, err
	}

	// Copy term options
	for _, to := range source.TermOptions {
		_, err := r.CreateTermOption(ctx, tenantID, mt.ID, to.DurationMonths,
			to.FeeTotal, to.FeeWeekly, to.FeeFortnightly, to.FeeMonthly, to.FeeQuarterly, to.AllowUpfront, to.NumPayments,
			to.CustomScheduleEnabled, to.CustomScheduleLabel, to.CustomScheduleAmount, to.CustomScheduleIntervalUnit, to.CustomScheduleIntervalCount, to.CustomScheduleNumPayments)
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
	customScheduleEnabled bool,
	customScheduleLabel *string,
	customScheduleAmount *float64,
	customScheduleIntervalUnit *string,
	customScheduleIntervalCount *int,
	customScheduleNumPayments *int,
) (*models.TermOption, error) {
	query := `
		INSERT INTO membership_term_options (tenant_id, membership_type_id, duration_months,
		    fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront, num_payments,
		    custom_schedule_enabled, custom_schedule_label, custom_schedule_amount,
		    custom_schedule_interval_unit, custom_schedule_interval_count, custom_schedule_num_payments)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, tenant_id, membership_type_id, duration_months,
		          fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront, num_payments,
		          custom_schedule_enabled, custom_schedule_label, custom_schedule_amount,
		          custom_schedule_interval_unit, custom_schedule_interval_count, custom_schedule_num_payments`

	var to models.TermOption
	err := r.db.Pool.QueryRow(ctx, query, tenantID, membershipTypeID, durationMonths,
		feeTotal, feeWeekly, feeFortnightly, feeMonthly, feeQuarterly, allowUpfront, numPayments,
		customScheduleEnabled, customScheduleLabel, customScheduleAmount, customScheduleIntervalUnit, customScheduleIntervalCount, customScheduleNumPayments).Scan(
		&to.ID, &to.TenantID, &to.MembershipTypeID, &to.DurationMonths,
		&to.FeeTotal, &to.FeeWeekly, &to.FeeFortnightly, &to.FeeMonthly, &to.FeeQuarterly, &to.AllowUpfront, &to.NumPayments,
		&to.CustomScheduleEnabled, &to.CustomScheduleLabel, &to.CustomScheduleAmount,
		&to.CustomScheduleIntervalUnit, &to.CustomScheduleIntervalCount, &to.CustomScheduleNumPayments)
	if err != nil {
		return nil, fmt.Errorf("creating term option: %w", err)
	}
	return &to, nil
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
	customScheduleEnabled bool,
	customScheduleLabel *string,
	customScheduleAmount *float64,
	customScheduleIntervalUnit *string,
	customScheduleIntervalCount *int,
	customScheduleNumPayments *int,
) (*models.TermOption, error) {
	query := `
		UPDATE membership_term_options SET duration_months=$3, fee_total=$4,
		    fee_weekly=$5, fee_fortnightly=$6, fee_monthly=$7, fee_quarterly=$8,
		    allow_upfront=$9, num_payments=$10, custom_schedule_enabled=$11,
		    custom_schedule_label=$12, custom_schedule_amount=$13,
		    custom_schedule_interval_unit=$14, custom_schedule_interval_count=$15,
		    custom_schedule_num_payments=$16, updated_at=NOW()
		WHERE id=$1 AND tenant_id=$2
		RETURNING id, tenant_id, membership_type_id, duration_months,
		          fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront, num_payments,
		          custom_schedule_enabled, custom_schedule_label, custom_schedule_amount,
		          custom_schedule_interval_unit, custom_schedule_interval_count, custom_schedule_num_payments`

	var to models.TermOption
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID, durationMonths,
		feeTotal, feeWeekly, feeFortnightly, feeMonthly, feeQuarterly, allowUpfront, numPayments,
		customScheduleEnabled, customScheduleLabel, customScheduleAmount, customScheduleIntervalUnit, customScheduleIntervalCount, customScheduleNumPayments).Scan(
		&to.ID, &to.TenantID, &to.MembershipTypeID, &to.DurationMonths,
		&to.FeeTotal, &to.FeeWeekly, &to.FeeFortnightly, &to.FeeMonthly, &to.FeeQuarterly, &to.AllowUpfront, &to.NumPayments,
		&to.CustomScheduleEnabled, &to.CustomScheduleLabel, &to.CustomScheduleAmount,
		&to.CustomScheduleIntervalUnit, &to.CustomScheduleIntervalCount, &to.CustomScheduleNumPayments)
	if err != nil {
		return nil, fmt.Errorf("updating term option: %w", err)
	}
	return &to, nil
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

// GetTermOption returns a single term option by ID.
func (r *MembershipTypeRepo) GetTermOption(ctx context.Context, termOptionID, tenantID uuid.UUID) (*models.TermOption, error) {
	query := `
		SELECT id, tenant_id, membership_type_id, duration_months,
		       fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront, num_payments,
		       custom_schedule_enabled, custom_schedule_label, custom_schedule_amount,
		       custom_schedule_interval_unit, custom_schedule_interval_count, custom_schedule_num_payments
		FROM membership_term_options
		WHERE id = $1 AND tenant_id = $2`
	var to models.TermOption
	err := r.db.Pool.QueryRow(ctx, query, termOptionID, tenantID).Scan(
		&to.ID, &to.TenantID, &to.MembershipTypeID, &to.DurationMonths,
		&to.FeeTotal, &to.FeeWeekly, &to.FeeFortnightly, &to.FeeMonthly, &to.FeeQuarterly, &to.AllowUpfront, &to.NumPayments,
		&to.CustomScheduleEnabled, &to.CustomScheduleLabel, &to.CustomScheduleAmount,
		&to.CustomScheduleIntervalUnit, &to.CustomScheduleIntervalCount, &to.CustomScheduleNumPayments)
	if err != nil {
		return nil, err
	}
	return &to, nil
}

func (r *MembershipTypeRepo) getTermOptions(ctx context.Context, membershipTypeID, tenantID uuid.UUID) ([]models.TermOption, error) {
	query := `
		SELECT id, tenant_id, membership_type_id, duration_months,
		       fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront, num_payments,
		       custom_schedule_enabled, custom_schedule_label, custom_schedule_amount,
		       custom_schedule_interval_unit, custom_schedule_interval_count, custom_schedule_num_payments
		FROM membership_term_options
		WHERE membership_type_id = $1 AND tenant_id = $2
		ORDER BY duration_months`

	rows, err := r.db.Pool.Query(ctx, query, membershipTypeID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var opts []models.TermOption
	for rows.Next() {
		var to models.TermOption
		if err := rows.Scan(&to.ID, &to.TenantID, &to.MembershipTypeID,
			&to.DurationMonths, &to.FeeTotal, &to.FeeWeekly,
			&to.FeeFortnightly, &to.FeeMonthly, &to.FeeQuarterly, &to.AllowUpfront, &to.NumPayments,
			&to.CustomScheduleEnabled, &to.CustomScheduleLabel, &to.CustomScheduleAmount,
			&to.CustomScheduleIntervalUnit, &to.CustomScheduleIntervalCount, &to.CustomScheduleNumPayments); err != nil {
			return nil, err
		}
		opts = append(opts, to)
	}
	return opts, rows.Err()
}

func (r *MembershipTypeRepo) findOne(ctx context.Context, where string, args ...interface{}) (*models.MembershipType, error) {
	query := `
		SELECT id, tenant_id, membership_name, membership_level, title, slug,
		       summary, description, inclusions, benefits,
		       top_image_url, currency_code, enabled_payment_frequencies, is_active, is_most_popular, sort_order, use_custom_thankyou,
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
		&mt.TopImageURL, &mt.CurrencyCode, enabledPaymentFrequenciesRaw, &mt.IsActive, &mt.IsMostPopular, &mt.SortOrder,
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
			updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, membership_name, membership_level, title, slug,
		          summary, description, inclusions, benefits,
		          top_image_url, currency_code, enabled_payment_frequencies, is_active, is_most_popular, sort_order, use_custom_thankyou,
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
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID, membershipName, membershipLevel, title, slug, summary, description, inclusions, benefits, topImageURL, currencyCode, sortOrder, enabledPaymentFrequenciesJSON).
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
