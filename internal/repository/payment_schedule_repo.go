package repository

import (
	"context"
	"fmt"
	"math"

	"clientroll/internal/database"
	"clientroll/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PaymentScheduleRepo handles CRUD for term_option_payment_schedules.
type PaymentScheduleRepo struct {
	db *database.Postgres
}

// NewPaymentScheduleRepo creates a new payment schedule repository.
func NewPaymentScheduleRepo(db *database.Postgres) *PaymentScheduleRepo {
	return &PaymentScheduleRepo{db: db}
}

// PaymentScheduleInput is the create/replace payload for a single schedule row.
// It mirrors models.PaymentSchedule but omits the server-managed fields
// (id, tenant_id, term_option_id, timestamps).
type PaymentScheduleInput struct {
	Kind          string   `json:"kind"`
	Label         *string  `json:"label"`
	Amount        float64  `json:"amount"`
	IntervalUnit  *string  `json:"interval_unit"`
	IntervalCount *int     `json:"interval_count"`
	NumPayments   *int     `json:"num_payments"`
	IsDefault     bool     `json:"is_default"`
	SortOrder     int      `json:"sort_order"`
}

// ListForTermOption returns every active (non-archived) schedule row for a term option.
func (r *PaymentScheduleRepo) ListForTermOption(ctx context.Context, termOptionID, tenantID uuid.UUID) ([]models.PaymentSchedule, error) {
	rows, err := r.db.Pool.Query(ctx, listSchedulesForTermOptionSQL, termOptionID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing payment schedules: %w", err)
	}
	defer rows.Close()
	return scanSchedules(rows)
}

// ListForTenant returns every schedule row for a tenant, grouped by term_option_id.
// Used for batch eager-loading from the membership type list path.
func (r *PaymentScheduleRepo) ListForTenant(ctx context.Context, tenantID uuid.UUID) (map[uuid.UUID][]models.PaymentSchedule, error) {
	rows, err := r.db.Pool.Query(ctx, listSchedulesForTenantSQL, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID][]models.PaymentSchedule)
	for rows.Next() {
		s, err := scanOneSchedule(rows)
		if err != nil {
			return nil, err
		}
		out[s.TermOptionID] = append(out[s.TermOptionID], s)
	}
	return out, rows.Err()
}

// ReplaceForTermOption atomically deletes existing rows for the term option
// and inserts the new set. The caller is responsible for validating input
// (see ValidateScheduleInputs).
func (r *PaymentScheduleRepo) ReplaceForTermOption(
	ctx context.Context,
	tenantID, termOptionID uuid.UUID,
	inputs []PaymentScheduleInput,
) ([]models.PaymentSchedule, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.replaceWithinTx(ctx, tx, tenantID, termOptionID, inputs); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return r.ListForTermOption(ctx, termOptionID, tenantID)
}

// ReplaceForTermOptionTx is the same as ReplaceForTermOption but participates
// in an externally managed transaction.
func (r *PaymentScheduleRepo) ReplaceForTermOptionTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, termOptionID uuid.UUID,
	inputs []PaymentScheduleInput,
) error {
	return r.replaceWithinTx(ctx, tx, tenantID, termOptionID, inputs)
}

func (r *PaymentScheduleRepo) replaceWithinTx(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, termOptionID uuid.UUID,
	inputs []PaymentScheduleInput,
) error {
	// Clear any membership FK references to these schedule rows so the
	// DELETE doesn't violate the memberships_payment_schedule_id_fkey constraint.
	if _, err := tx.Exec(ctx,
		`UPDATE memberships SET payment_schedule_id = NULL
		 WHERE payment_schedule_id IN (
		   SELECT id FROM term_option_payment_schedules
		   WHERE term_option_id = $1 AND tenant_id = $2
		 )`,
		termOptionID, tenantID); err != nil {
		return fmt.Errorf("clear membership schedule refs: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM term_option_payment_schedules
		 WHERE term_option_id = $1 AND tenant_id = $2`,
		termOptionID, tenantID); err != nil {
		return fmt.Errorf("clear existing schedules: %w", err)
	}
	if len(inputs) == 0 {
		return nil
	}
	const insertSQL = `
		INSERT INTO term_option_payment_schedules
			(tenant_id, term_option_id, kind, label, amount,
			 interval_unit, interval_count, num_payments,
			 is_default, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	for _, in := range inputs {
		if _, err := tx.Exec(ctx, insertSQL,
			tenantID, termOptionID, in.Kind, in.Label, in.Amount,
			in.IntervalUnit, in.IntervalCount, in.NumPayments,
			in.IsDefault, in.SortOrder); err != nil {
			return fmt.Errorf("insert schedule %s: %w", in.Kind, err)
		}
	}
	return nil
}

// ValidateScheduleInputs checks the basic shape constraints the DB also
// enforces, plus the soft rule that there is at most one default. Returns
// a user-facing error message on the first failure.
func ValidateScheduleInputs(inputs []PaymentScheduleInput) error {
	defaultCount := 0
	for i, in := range inputs {
		switch in.Kind {
		case "upfront":
			if in.IntervalUnit != nil || in.IntervalCount != nil {
				return fmt.Errorf("schedule[%d]: upfront kind must not carry an interval", i)
			}
		case "recurring", "instalments":
			if in.IntervalUnit == nil || (*in.IntervalUnit != "week" && *in.IntervalUnit != "month") {
				return fmt.Errorf("schedule[%d]: interval_unit must be 'week' or 'month'", i)
			}
			if in.IntervalCount == nil || *in.IntervalCount <= 0 {
				return fmt.Errorf("schedule[%d]: interval_count must be > 0", i)
			}
			if in.Kind == "instalments" && (in.NumPayments == nil || *in.NumPayments <= 0) {
				return fmt.Errorf("schedule[%d]: num_payments is required for instalments", i)
			}
		default:
			return fmt.Errorf("schedule[%d]: unsupported kind %q", i, in.Kind)
		}
		if in.Amount < 0 {
			return fmt.Errorf("schedule[%d]: amount must be >= 0", i)
		}
		if in.IsDefault {
			defaultCount++
		}
	}
	if defaultCount > 1 {
		return fmt.Errorf("only one payment schedule can be the default")
	}
	return nil
}

// LegacyFeesToSchedules turns the old per-frequency fee_* + custom_schedule_*
// inputs into a canonical slice of PaymentScheduleInput rows. This is the
// single source of truth used both by the migration backfill and by the
// legacy admin path during the dual-write soak window.
func LegacyFeesToSchedules(
	durationMonths int,
	feeTotal float64,
	feeWeekly, feeFortnightly, feeMonthly, feeQuarterly *float64,
	allowUpfront bool,
	continueAfterTerm bool,
	customScheduleEnabled bool,
	customScheduleLabel *string,
	customScheduleAmount *float64,
	customScheduleIntervalUnit *string,
	customScheduleIntervalCount *int,
	customScheduleNumPayments *int,
) []PaymentScheduleInput {
	out := make([]PaymentScheduleInput, 0, 6)

	if allowUpfront {
		out = append(out, PaymentScheduleInput{
			Kind:        "upfront",
			Amount:      math.Max(0, feeTotal),
			NumPayments: intPtr(1),
			IsDefault:   true,
			SortOrder:   0,
		})
	}
	addRecurring := func(amount *float64, intervalUnit string, intervalCount, defaultNumPayments int, sortOrder int) {
		if amount == nil || *amount <= 0 {
			return
		}
		var nump *int
		if !continueAfterTerm && defaultNumPayments > 0 {
			n := defaultNumPayments
			nump = &n
		}
		unit := intervalUnit
		count := intervalCount
		out = append(out, PaymentScheduleInput{
			Kind:          "recurring",
			Amount:        *amount,
			IntervalUnit:  &unit,
			IntervalCount: &count,
			NumPayments:   nump,
			SortOrder:     sortOrder,
		})
	}
	weeklyCount := 0
	if durationMonths > 0 {
		weeklyCount = int(math.Max(1, math.Round(float64(durationMonths)*52.0/12.0)))
	}
	fortnightlyCount := 0
	if durationMonths > 0 {
		fortnightlyCount = int(math.Max(1, math.Round(float64(durationMonths)*26.0/12.0)))
	}
	quarterlyCount := 0
	if durationMonths > 0 {
		quarterlyCount = int(math.Max(1, math.Ceil(float64(durationMonths)/3.0)))
	}

	addRecurring(feeWeekly, "week", 1, weeklyCount, 10)
	addRecurring(feeFortnightly, "week", 2, fortnightlyCount, 20)
	addRecurring(feeMonthly, "month", 1, durationMonths, 30)
	addRecurring(feeQuarterly, "month", 3, quarterlyCount, 40)

	if customScheduleEnabled &&
		customScheduleAmount != nil && *customScheduleAmount > 0 &&
		customScheduleIntervalUnit != nil && customScheduleIntervalCount != nil {
		out = append(out, PaymentScheduleInput{
			Kind:          "instalments",
			Label:         customScheduleLabel,
			Amount:        *customScheduleAmount,
			IntervalUnit:  customScheduleIntervalUnit,
			IntervalCount: customScheduleIntervalCount,
			NumPayments:   customScheduleNumPayments,
			SortOrder:     50,
		})
	}

	// Promote the lowest sort_order entry to default if no upfront row exists.
	if !allowUpfront && len(out) > 0 {
		out[0].IsDefault = true
	}
	return out
}

func intPtr(v int) *int { return &v }

const listSchedulesForTermOptionSQL = `
	SELECT id, tenant_id, term_option_id, kind, label, amount,
	       interval_unit, interval_count, num_payments,
	       is_default, sort_order, archived_at, created_at, updated_at
	FROM term_option_payment_schedules
	WHERE term_option_id = $1 AND tenant_id = $2 AND archived_at IS NULL
	ORDER BY sort_order, created_at`

const listSchedulesForTenantSQL = `
	SELECT id, tenant_id, term_option_id, kind, label, amount,
	       interval_unit, interval_count, num_payments,
	       is_default, sort_order, archived_at, created_at, updated_at
	FROM term_option_payment_schedules
	WHERE tenant_id = $1 AND archived_at IS NULL
	ORDER BY term_option_id, sort_order, created_at`

func scanSchedules(rows pgx.Rows) ([]models.PaymentSchedule, error) {
	var out []models.PaymentSchedule
	for rows.Next() {
		s, err := scanOneSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func scanOneSchedule(rows pgx.Rows) (models.PaymentSchedule, error) {
	var s models.PaymentSchedule
	err := rows.Scan(
		&s.ID, &s.TenantID, &s.TermOptionID, &s.Kind, &s.Label, &s.Amount,
		&s.IntervalUnit, &s.IntervalCount, &s.NumPayments,
		&s.IsDefault, &s.SortOrder, &s.ArchivedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	return s, err
}
