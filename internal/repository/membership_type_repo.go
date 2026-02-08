package repository

import (
	"context"
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

// NewMembershipTypeRepo creates a new membership type repository.
func NewMembershipTypeRepo(db *database.Postgres) *MembershipTypeRepo {
	return &MembershipTypeRepo{db: db}
}

// List returns all membership types for a tenant.
func (r *MembershipTypeRepo) List(ctx context.Context, tenantID uuid.UUID) ([]models.MembershipType, error) {
	query := `
		SELECT id, tenant_id, title, slug, description, inclusions, top_image_url,
		       is_active, intrinsic_value, total_value, sort_order, use_custom_thankyou,
		       thankyou_heading, thankyou_body, thankyou_image_url, thankyou_redirect_url,
		       created_at, updated_at
		FROM membership_types WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY sort_order, created_at`

	rows, err := r.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing membership types: %w", err)
	}
	defer rows.Close()
	return r.scanMany(rows)
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
func (r *MembershipTypeRepo) Create(ctx context.Context, tenantID uuid.UUID, title, slug string, description, inclusions, topImageURL *string, totalValue float64, sortOrder int) (*models.MembershipType, error) {
	query := `
		INSERT INTO membership_types (tenant_id, title, slug, description, inclusions,
		    top_image_url, intrinsic_value, total_value, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8)
		RETURNING id, tenant_id, title, slug, description, inclusions, top_image_url,
		          is_active, intrinsic_value, total_value, sort_order, use_custom_thankyou,
		          thankyou_heading, thankyou_body, thankyou_image_url, thankyou_redirect_url,
		          created_at, updated_at`

	var mt models.MembershipType
	err := r.db.Pool.QueryRow(ctx, query, tenantID, title, slug, description,
		inclusions, topImageURL, totalValue, sortOrder).Scan(r.scanFields(&mt)...)
	if err != nil {
		return nil, fmt.Errorf("creating membership type: %w", err)
	}
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

	mt, err := r.Create(ctx, tenantID, newTitle, newSlug,
		source.Description, source.Inclusions, source.TopImageURL,
		source.TotalValue, source.SortOrder+1)
	if err != nil {
		return nil, err
	}

	// Copy term options
	for _, to := range source.TermOptions {
		_, err := r.CreateTermOption(ctx, tenantID, mt.ID, to.DurationMonths,
			to.FeeTotal, to.FeeWeekly, to.FeeFortnightly, to.FeeMonthly, to.FeeQuarterly, to.AllowUpfront, to.NumPayments)
		if err != nil {
			return nil, fmt.Errorf("duplicating term option: %w", err)
		}
	}

	return r.FindByID(ctx, mt.ID, tenantID)
}

// CreateTermOption adds a term option to a membership type.
func (r *MembershipTypeRepo) CreateTermOption(ctx context.Context, tenantID, membershipTypeID uuid.UUID, durationMonths int, feeTotal float64, feeWeekly, feeFortnightly, feeMonthly, feeQuarterly *float64, allowUpfront bool, numPayments *int) (*models.TermOption, error) {
	query := `
		INSERT INTO membership_term_options (tenant_id, membership_type_id, duration_months,
		    fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront, num_payments)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, tenant_id, membership_type_id, duration_months,
		          fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront, num_payments`

	var to models.TermOption
	err := r.db.Pool.QueryRow(ctx, query, tenantID, membershipTypeID, durationMonths,
		feeTotal, feeWeekly, feeFortnightly, feeMonthly, feeQuarterly, allowUpfront, numPayments).Scan(
		&to.ID, &to.TenantID, &to.MembershipTypeID, &to.DurationMonths,
		&to.FeeTotal, &to.FeeWeekly, &to.FeeFortnightly, &to.FeeMonthly, &to.FeeQuarterly, &to.AllowUpfront, &to.NumPayments)
	if err != nil {
		return nil, fmt.Errorf("creating term option: %w", err)
	}
	return &to, nil
}

// UpdateTermOption updates a term option.
func (r *MembershipTypeRepo) UpdateTermOption(ctx context.Context, id, tenantID uuid.UUID, durationMonths int, feeTotal float64, feeWeekly, feeFortnightly, feeMonthly, feeQuarterly *float64, allowUpfront bool, numPayments *int) (*models.TermOption, error) {
	query := `
		UPDATE membership_term_options SET duration_months=$3, fee_total=$4,
		    fee_weekly=$5, fee_fortnightly=$6, fee_monthly=$7, fee_quarterly=$8, allow_upfront=$9, num_payments=$10, updated_at=NOW()
		WHERE id=$1 AND tenant_id=$2
		RETURNING id, tenant_id, membership_type_id, duration_months,
		          fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront, num_payments`

	var to models.TermOption
	err := r.db.Pool.QueryRow(ctx, query, id, tenantID, durationMonths,
		feeTotal, feeWeekly, feeFortnightly, feeMonthly, feeQuarterly, allowUpfront, numPayments).Scan(
		&to.ID, &to.TenantID, &to.MembershipTypeID, &to.DurationMonths,
		&to.FeeTotal, &to.FeeWeekly, &to.FeeFortnightly, &to.FeeMonthly, &to.FeeQuarterly, &to.AllowUpfront, &to.NumPayments)
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
		"UPDATE membership_types SET deleted_at = NOW(), is_active = false, updated_at = NOW() WHERE id=$1 AND tenant_id=$2", id, tenantID)
	return err
}

// Archive sets is_active = false (soft delete). Existing memberships are unaffected.
func (r *MembershipTypeRepo) Archive(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx,
		"UPDATE membership_types SET is_active = false, updated_at = NOW() WHERE id=$1 AND tenant_id=$2", id, tenantID)
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

func (r *MembershipTypeRepo) getTermOptions(ctx context.Context, membershipTypeID, tenantID uuid.UUID) ([]models.TermOption, error) {
	query := `
		SELECT id, tenant_id, membership_type_id, duration_months,
		       fee_total, fee_weekly, fee_fortnightly, fee_monthly, fee_quarterly, allow_upfront, num_payments
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
			&to.FeeFortnightly, &to.FeeMonthly, &to.FeeQuarterly, &to.AllowUpfront, &to.NumPayments); err != nil {
			return nil, err
		}
		opts = append(opts, to)
	}
	return opts, rows.Err()
}

func (r *MembershipTypeRepo) findOne(ctx context.Context, where string, args ...interface{}) (*models.MembershipType, error) {
	query := `
		SELECT id, tenant_id, title, slug, description, inclusions, top_image_url,
		       is_active, intrinsic_value, total_value, sort_order, use_custom_thankyou,
		       thankyou_heading, thankyou_body, thankyou_image_url, thankyou_redirect_url,
		       created_at, updated_at
		FROM membership_types WHERE ` + where

	var mt models.MembershipType
	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(r.scanFields(&mt)...)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("finding membership type: %w", err)
	}
	return &mt, nil
}

func (r *MembershipTypeRepo) scanFields(mt *models.MembershipType) []interface{} {
	return []interface{}{
		&mt.ID, &mt.TenantID, &mt.Title, &mt.Slug, &mt.Description,
		&mt.Inclusions, &mt.TopImageURL, &mt.IsActive, &mt.IntrinsicValue, &mt.TotalValue, &mt.SortOrder,
		&mt.UseCustomThankyou, &mt.ThankyouHeading, &mt.ThankyouBody,
		&mt.ThankyouImageURL, &mt.ThankyouRedirectURL, &mt.CreatedAt, &mt.UpdatedAt,
	}
}

func (r *MembershipTypeRepo) scanMany(rows pgx.Rows) ([]models.MembershipType, error) {
	var items []models.MembershipType
	for rows.Next() {
		var mt models.MembershipType
		if err := rows.Scan(r.scanFields(&mt)...); err != nil {
			return nil, fmt.Errorf("scanning membership type: %w", err)
		}
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
