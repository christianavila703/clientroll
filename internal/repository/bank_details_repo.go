package repository

import (
	"context"

	"clientroll/internal/database"

	"github.com/google/uuid"
)

// BankDetail represents a stored bank detail record (last 4 digits only).
type BankDetail struct {
	ID                      uuid.UUID `json:"id"`
	TenantID                uuid.UUID `json:"tenant_id"`
	ClientID                uuid.UUID `json:"client_id"`
	AccountHolderName       string    `json:"account_holder_name"`
	BSB                     string    `json:"bsb"`
	AccountNumberLast4      string    `json:"account_number_last4"`
	InstitutionName         string    `json:"institution_name"`
	GoCardlessBankAccountID string    `json:"gocardless_bank_account_id,omitempty"`
}

// BankDetailsRepo handles bank details database operations.
type BankDetailsRepo struct {
	db *database.Postgres
}

// NewBankDetailsRepo creates a new bank details repository.
func NewBankDetailsRepo(db *database.Postgres) *BankDetailsRepo {
	return &BankDetailsRepo{db: db}
}

// Create stores bank details (last 4 digits of account number only).
func (r *BankDetailsRepo) Create(ctx context.Context, tenantID, clientID uuid.UUID, holderName, bsb, accountNumberLast4, institutionName, gcBankAccountID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO bank_details (tenant_id, client_id, account_holder_name, bsb, account_number_last4, institution_name, gocardless_bank_account_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		tenantID, clientID, holderName, bsb, accountNumberLast4, institutionName, gcBankAccountID)
	return err
}

// GetByClient returns bank details for a client.
func (r *BankDetailsRepo) GetByClient(ctx context.Context, clientID, tenantID uuid.UUID) ([]BankDetail, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, tenant_id, client_id, account_holder_name, bsb, account_number_last4, institution_name, COALESCE(gocardless_bank_account_id, '')
		 FROM bank_details WHERE client_id = $1 AND tenant_id = $2 ORDER BY created_at DESC`,
		clientID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var details []BankDetail
	for rows.Next() {
		var d BankDetail
		if err := rows.Scan(&d.ID, &d.TenantID, &d.ClientID, &d.AccountHolderName, &d.BSB, &d.AccountNumberLast4, &d.InstitutionName, &d.GoCardlessBankAccountID); err != nil {
			return nil, err
		}
		details = append(details, d)
	}
	return details, rows.Err()
}

// RedirectFlowSession maps a redirect flow session_token to a client+tenant.
type RedirectFlowSession struct {
	ID           uuid.UUID `json:"id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	ClientID     uuid.UUID `json:"client_id"`
	SessionToken string    `json:"session_token"`
	FlowID       string    `json:"flow_id"`
}

// RedirectFlowSessionRepo handles redirect flow session DB operations.
type RedirectFlowSessionRepo struct {
	db *database.Postgres
}

// NewRedirectFlowSessionRepo creates a new repo.
func NewRedirectFlowSessionRepo(db *database.Postgres) *RedirectFlowSessionRepo {
	return &RedirectFlowSessionRepo{db: db}
}

// Create stores a redirect flow session.
func (r *RedirectFlowSessionRepo) Create(ctx context.Context, tenantID, clientID uuid.UUID, sessionToken, flowID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO redirect_flow_sessions (tenant_id, client_id, session_token, flow_id) VALUES ($1, $2, $3, $4)`,
		tenantID, clientID, sessionToken, flowID)
	return err
}

// FindBySessionToken retrieves a session by token.
func (r *RedirectFlowSessionRepo) FindBySessionToken(ctx context.Context, sessionToken string) (*RedirectFlowSession, error) {
	var s RedirectFlowSession
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, tenant_id, client_id, session_token, COALESCE(flow_id, '') FROM redirect_flow_sessions WHERE session_token = $1`,
		sessionToken).Scan(&s.ID, &s.TenantID, &s.ClientID, &s.SessionToken, &s.FlowID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Delete removes a session after use.
func (r *RedirectFlowSessionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM redirect_flow_sessions WHERE id = $1`, id)
	return err
}
