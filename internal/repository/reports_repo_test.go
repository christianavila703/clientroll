package repository

import "testing"

func TestCanonicalReportFormulaExpressions(t *testing.T) {
	if reportCollectedMembershipExpr != "amount" {
		t.Fatalf("unexpected membership collected expression: %q", reportCollectedMembershipExpr)
	}
	if reportCollectedServiceExpr != "amount + tax_amount + processing_fee_amount" {
		t.Fatalf("unexpected service collected expression: %q", reportCollectedServiceExpr)
	}
	if reportTaxMembershipExpr != "COALESCE(tax_amount, 0)" || reportTaxServiceExpr != "COALESCE(tax_amount, 0)" {
		t.Fatalf("unexpected tax expressions: membership=%q service=%q", reportTaxMembershipExpr, reportTaxServiceExpr)
	}
	if reportSurchargeMembershipExpr != "COALESCE(processing_fee_amount, 0)" || reportSurchargeServiceExpr != "COALESCE(processing_fee_amount, 0)" {
		t.Fatalf("unexpected surcharge expressions: membership=%q service=%q", reportSurchargeMembershipExpr, reportSurchargeServiceExpr)
	}
	if reportToOrgMembershipExpr != "COALESCE(base_amount, 0) + COALESCE(tax_amount, 0)" {
		t.Fatalf("unexpected membership to-organisation expression: %q", reportToOrgMembershipExpr)
	}
	if reportToOrgServiceExpr != "amount + tax_amount" {
		t.Fatalf("unexpected service to-organisation expression: %q", reportToOrgServiceExpr)
	}
}

func TestApplyDashboardLegacyAliases(t *testing.T) {
	s := DashboardSummary{
		MonthCollectedAmount:      120.5,
		MonthToOrganisationAmount: 115.0,
	}
	applyDashboardLegacyAliases(&s)
	if s.MonthRevenue != 120.5 {
		t.Fatalf("expected month_revenue alias to be 120.5, got %v", s.MonthRevenue)
	}
	if s.OrganisationMonthRevenue != 115.0 {
		t.Fatalf("expected organisation_month_revenue alias to be 115.0, got %v", s.OrganisationMonthRevenue)
	}
}

func TestApplyComprehensiveLegacyAliases(t *testing.T) {
	r := &ComprehensiveReport{
		MonthCollectedAmount:          100,
		PrevMonthCollectedAmount:      90,
		TotalCollectedAmount:          1000,
		MonthToOrganisationAmount:     95,
		PrevMonthToOrganisationAmount: 85,
		TotalToOrganisationAmount:     930,
	}
	applyComprehensiveLegacyAliases(r)
	if r.MonthRevenue != 100 || r.PrevMonthRevenue != 90 || r.TotalRevenue != 1000 {
		t.Fatalf("collected aliases not applied correctly: month=%v prev=%v total=%v", r.MonthRevenue, r.PrevMonthRevenue, r.TotalRevenue)
	}
	if r.OrganisationMonthRevenue != 95 || r.OrganisationPrevMonthRevenue != 85 || r.OrganisationTotalRevenue != 930 {
		t.Fatalf("organisation aliases not applied correctly: month=%v prev=%v total=%v", r.OrganisationMonthRevenue, r.OrganisationPrevMonthRevenue, r.OrganisationTotalRevenue)
	}
}
