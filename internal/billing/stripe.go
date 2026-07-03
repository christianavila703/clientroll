package billing

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/billingportal/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/price"
	"github.com/stripe/stripe-go/v82/product"
	"github.com/stripe/stripe-go/v82/subscription"
)

// StripeBillingProvider implements PlatformBillingProvider for Stripe.
type StripeBillingProvider struct {
	publishableKey string
	secretKey      string
	webhookSecret  string
}

// NewStripeBillingProvider creates a new Stripe billing provider.
func NewStripeBillingProvider(publishableKey, secretKey, webhookSecret string) *StripeBillingProvider {
	return &StripeBillingProvider{
		publishableKey: publishableKey,
		secretKey:      secretKey,
		webhookSecret:  webhookSecret,
	}
}

func (s *StripeBillingProvider) CreateSubscription(ctx context.Context, input SubscriptionInput) (string, error) {
	if input.StripePriceID == "" {
		return "", fmt.Errorf("StripePriceID is required")
	}
	if input.AdminEmail == "" {
		return "", fmt.Errorf("admin email is required")
	}

	stripe.Key = s.secretKey

	// Create customer (store tenant_id in metadata for webhook routing).
	cParams := &stripe.CustomerParams{
		Email: stripe.String(input.AdminEmail),
		Name:  stripe.String(input.TenantName),
		Metadata: map[string]string{
			"tenant_id": input.TenantID,
		},
	}
	cParams.Context = ctx

	c, err := customer.New(cParams)
	if err != nil {
		return "", fmt.Errorf("creating Stripe customer: %w", err)
	}

	// Create subscription. This does NOT collect a payment method by itself.
	// The application should prefer Checkout for initial card collection;
	// this exists for administrative/backfill use.
	sParams := &stripe.SubscriptionParams{
		Customer: stripe.String(c.ID),
		Items: []*stripe.SubscriptionItemsParams{
			{Price: stripe.String(input.StripePriceID), Quantity: stripe.Int64(0)},
		},
		Metadata: map[string]string{
			"tenant_id": input.TenantID,
		},
	}
	sParams.Context = ctx
	sParams.AddExpand("items.data.price")
	sParams.AddExpand("latest_invoice.payment_intent")

	sub, err := subscription.New(sParams)
	if err != nil {
		return "", fmt.Errorf("creating Stripe subscription: %w", err)
	}
	return sub.ID, nil
}

func (s *StripeBillingProvider) ReportUsage(ctx context.Context, subscriptionID string, memberCount int) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscriptionID is required")
	}
	if memberCount < 0 {
		return fmt.Errorf("memberCount cannot be negative")
	}

	stripe.Key = s.secretKey

	// Update subscription quantity for the first item (single-item subscription model).
	// This supports “$X per active member per month” licensed billing.
	getParams := &stripe.SubscriptionParams{}
	getParams.Context = ctx
	getParams.AddExpand("items.data.price")
	sub, err := subscription.Get(subscriptionID, getParams)
	if err != nil {
		return fmt.Errorf("fetching Stripe subscription: %w", err)
	}
	if sub.Items == nil || len(sub.Items.Data) == 0 {
		return fmt.Errorf("subscription has no items")
	}
	itemID := sub.Items.Data[0].ID

	up := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{ID: stripe.String(itemID), Quantity: stripe.Int64(int64(memberCount))},
		},
	}
	up.Context = ctx
	_, err = subscription.Update(subscriptionID, up)
	if err != nil {
		return fmt.Errorf("updating subscription quantity: %w", err)
	}
	return nil
}

func (s *StripeBillingProvider) GetSubscriptionStatus(ctx context.Context, subscriptionID string) (SubscriptionStatus, error) {
	if subscriptionID == "" {
		return SubscriptionStatus{}, fmt.Errorf("subscriptionID is required")
	}

	stripe.Key = s.secretKey

	params := &stripe.SubscriptionParams{}
	params.Context = ctx
	params.AddExpand("items.data.price")
	sub, err := subscription.Get(subscriptionID, params)
	if err != nil {
		return SubscriptionStatus{}, fmt.Errorf("fetching Stripe subscription: %w", err)
	}

	st := SubscriptionStatus{
		ID:     sub.ID,
		Status: string(sub.Status),
	}

	if sub.Items != nil && len(sub.Items.Data) > 0 {
		item := sub.Items.Data[0]
		if item.CurrentPeriodEnd > 0 {
			st.CurrentPeriodEnd = time.Unix(item.CurrentPeriodEnd, 0).UTC().Format("2006-01-02")
		}
		st.MemberCount = int(item.Quantity)
		if item.Price != nil && item.Price.UnitAmount != 0 {
			st.MonthlyAmount = (float64(item.Price.UnitAmount) / 100.0) * float64(st.MemberCount)
		}
	}

	return st, nil
}

func (s *StripeBillingProvider) CancelSubscription(ctx context.Context, subscriptionID string) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscriptionID is required")
	}
	stripe.Key = s.secretKey

	params := &stripe.SubscriptionCancelParams{}
	params.Context = ctx
	_, err := subscription.Cancel(subscriptionID, params)
	if err != nil {
		return fmt.Errorf("canceling Stripe subscription: %w", err)
	}
	return nil
}

// PlatformProductSyncInput configures create/update of the platform billing Stripe product + price.
type PlatformProductSyncInput struct {
	ProductName       string
	PricePerMember    float64
	Currency          string
	ExistingProductID string
	ExistingPriceID   string
}

// PlatformProductSyncResult is returned after syncing the Stripe product and price.
type PlatformProductSyncResult struct {
	ProductID       string `json:"product_id"`
	PriceID         string `json:"price_id"`
	CreatedProduct  bool   `json:"created_product"`
	CreatedPrice    bool   `json:"created_price"`
	ArchivedPriceID string `json:"archived_price_id,omitempty"`
}

// SyncPlatformProduct creates or updates the Stripe product and recurring per-member price.
// Stripe prices are immutable; when the amount changes a new price is created and the old one archived.
func (s *StripeBillingProvider) SyncPlatformProduct(ctx context.Context, in PlatformProductSyncInput) (PlatformProductSyncResult, error) {
	if s.secretKey == "" {
		return PlatformProductSyncResult{}, fmt.Errorf("stripe secret key is required")
	}
	if in.PricePerMember <= 0 {
		return PlatformProductSyncResult{}, fmt.Errorf("price per member must be greater than zero")
	}

	currency := strings.ToLower(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "usd"
	}
	productName := strings.TrimSpace(in.ProductName)
	if productName == "" {
		productName = "Clientroll Platform — Per Active Member"
	}

	unitAmount := int64(math.Round(in.PricePerMember * 100))
	if unitAmount <= 0 {
		return PlatformProductSyncResult{}, fmt.Errorf("price per member is too small")
	}

	stripe.Key = s.secretKey

	var result PlatformProductSyncResult
	productID := strings.TrimSpace(in.ExistingProductID)

	if productID == "" {
		pParams := &stripe.ProductParams{
			Name: stripe.String(productName),
			Metadata: map[string]string{
				"clientroll": "platform_billing",
			},
		}
		pParams.Context = ctx
		prod, err := product.New(pParams)
		if err != nil {
			return PlatformProductSyncResult{}, fmt.Errorf("creating Stripe product: %w", err)
		}
		productID = prod.ID
		result.CreatedProduct = true
	} else {
		uParams := &stripe.ProductParams{
			Name: stripe.String(productName),
		}
		uParams.Context = ctx
		if _, err := product.Update(productID, uParams); err != nil {
			return PlatformProductSyncResult{}, fmt.Errorf("updating Stripe product: %w", err)
		}
	}
	result.ProductID = productID

	existingPriceID := strings.TrimSpace(in.ExistingPriceID)
	needNewPrice := true
	if existingPriceID != "" {
		getParams := &stripe.PriceParams{}
		getParams.Context = ctx
		existing, err := price.Get(existingPriceID, getParams)
		if err == nil &&
			existing.UnitAmount == unitAmount &&
			strings.EqualFold(string(existing.Currency), currency) &&
			existing.Recurring != nil &&
			existing.Recurring.Interval == stripe.PriceRecurringIntervalMonth {
			needNewPrice = false
			result.PriceID = existingPriceID
		}
	}

	if needNewPrice {
		prParams := &stripe.PriceParams{
			Currency:   stripe.String(currency),
			Product:    stripe.String(productID),
			UnitAmount: stripe.Int64(unitAmount),
			Recurring: &stripe.PriceRecurringParams{
				Interval:  stripe.String(string(stripe.PriceRecurringIntervalMonth)),
				UsageType: stripe.String(string(stripe.PriceRecurringUsageTypeLicensed)),
			},
			Nickname: stripe.String(fmt.Sprintf("$%.2f per active member / month", in.PricePerMember)),
			Metadata: map[string]string{
				"clientroll": "platform_billing",
			},
		}
		prParams.Context = ctx
		pr, err := price.New(prParams)
		if err != nil {
			return PlatformProductSyncResult{}, fmt.Errorf("creating Stripe price: %w", err)
		}
		result.PriceID = pr.ID
		result.CreatedPrice = true

		if existingPriceID != "" && existingPriceID != pr.ID {
			archParams := &stripe.PriceParams{Active: stripe.Bool(false)}
			archParams.Context = ctx
			if _, err := price.Update(existingPriceID, archParams); err != nil {
				return result, fmt.Errorf("created price %s but failed to archive %s: %w", pr.ID, existingPriceID, err)
			}
			result.ArchivedPriceID = existingPriceID
		}
	}

	return result, nil
}

// SubscriptionPriceMigrationResult summarizes bulk subscription migration after a platform price change.
type SubscriptionPriceMigrationResult struct {
	Total    int      `json:"total"`
	Migrated int      `json:"migrated"`
	Skipped  int      `json:"skipped"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
}

// MigrateSubscriptionToPrice moves a subscription onto newPriceID when its line item uses fromPriceID.
// Returns skipped=true when already on newPriceID or not on the expected old price.
func (s *StripeBillingProvider) MigrateSubscriptionToPrice(ctx context.Context, subscriptionID, newPriceID, fromPriceID string) (skipped bool, err error) {
	if subscriptionID == "" || newPriceID == "" {
		return false, fmt.Errorf("subscriptionID and newPriceID are required")
	}

	stripe.Key = s.secretKey

	getParams := &stripe.SubscriptionParams{}
	getParams.Context = ctx
	getParams.AddExpand("items.data.price")
	sub, err := subscription.Get(subscriptionID, getParams)
	if err != nil {
		return false, fmt.Errorf("fetching subscription: %w", err)
	}
	if sub.Items == nil || len(sub.Items.Data) == 0 {
		return false, fmt.Errorf("subscription has no items")
	}

	item := sub.Items.Data[0]
	currentPriceID := ""
	if item.Price != nil {
		currentPriceID = item.Price.ID
	}
	if currentPriceID == newPriceID {
		return true, nil
	}
	if fromPriceID != "" && currentPriceID != fromPriceID {
		return true, nil
	}

	up := &stripe.SubscriptionParams{
		Items: []*stripe.SubscriptionItemsParams{
			{
				ID:       stripe.String(item.ID),
				Price:    stripe.String(newPriceID),
				Quantity: stripe.Int64(item.Quantity),
			},
		},
		ProrationBehavior: stripe.String("none"),
	}
	up.Context = ctx
	if _, err := subscription.Update(subscriptionID, up); err != nil {
		return false, fmt.Errorf("updating subscription price: %w", err)
	}
	return false, nil
}

func (s *StripeBillingProvider) CreatePortalSession(ctx context.Context, customerID string) (string, error) {
	if customerID == "" {
		return "", fmt.Errorf("customerID is required")
	}
	stripe.Key = s.secretKey

	params := &stripe.BillingPortalSessionParams{
		Customer: stripe.String(customerID),
	}
	params.Context = ctx
	// Return URL is set by caller (preferred). If left empty, Stripe uses default portal settings.

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("creating Stripe billing portal session: %w", err)
	}
	return sess.URL, nil
}
