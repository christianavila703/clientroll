import { Routes, Route, Navigate } from "react-router-dom";
import { AuthProvider } from "./store/AuthContext";
import AppShell from "./components/layout/AppShell";
import LoginPage from "./components/auth/LoginPage";
import RegisterPage from "./components/auth/RegisterPage";
import ForgotPasswordPage from "./components/auth/ForgotPasswordPage";
import ResetPasswordPage from "./components/auth/ResetPasswordPage";
import VerifyEmailPage from "./components/auth/VerifyEmailPage";
import DashboardPage from "./components/dashboard/DashboardPage";
import SettingsPage from "./components/settings/SettingsPage";
import StaffPage from "./components/staff/StaffPage";
import PlatformDashboard from "./components/platform/PlatformDashboard";
import ClientsPage from "./components/clients/ClientsPage";
import MembershipTypesPage from "./components/memberships/MembershipTypesPage";
import MembershipTypeDetail from "./components/memberships/MembershipTypeDetail";
import ActiveMembershipsPage from "./components/memberships/ActiveMembershipsPage";
import SignupPage from "./components/signup/SignupPage";
import PaymentsPage from "./components/payments/PaymentsPage";
import ReportsPage from "./components/reports/ReportsPage";
import TermsPage from "./components/terms/TermsPage";
import EmailTemplatesPage from "./components/settings/EmailTemplatesPage";
import BonusesPage from "./components/bonuses/BonusesPage";
import PlaceholderPage from "./components/common/PlaceholderPage";

function App() {
  return (
    <AuthProvider>
      <Routes>
        {/* Public routes */}
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
        <Route path="/verify-email" element={<VerifyEmailPage />} />

        {/* Public membership signup */}
        <Route path="/join/:tenantSlug" element={<SignupPage />} />
        <Route path="/:tenantSlug/:membershipSlug" element={<SignupPage />} />

        {/* Authenticated tenant routes */}
        <Route element={<AppShell />}>
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/clients" element={<ClientsPage />} />
          <Route path="/active-memberships" element={<ActiveMembershipsPage />} />
          <Route path="/memberships" element={<MembershipTypesPage />} />
          <Route path="/memberships/:mtId" element={<MembershipTypeDetail />} />
          <Route path="/bonuses" element={<BonusesPage />} />
          <Route path="/terms" element={<TermsPage />} />
          <Route path="/payments" element={<PaymentsPage />} />
          <Route path="/reports" element={<ReportsPage />} />
          <Route path="/staff" element={<StaffPage />} />
          <Route path="/email-templates" element={<EmailTemplatesPage />} />
          <Route path="/settings" element={<SettingsPage />} />

          {/* Platform admin routes */}
          <Route path="/platform" element={<PlatformDashboard />} />
          <Route
            path="/platform/tenants"
            element={<PlatformDashboard />}
          />
          <Route
            path="/platform/billing"
            element={
              <PlaceholderPage
                title="Platform Billing"
                description="Manage tenant subscriptions and usage"
                phase="Phase 8"
              />
            }
          />
        </Route>

        {/* Default redirect */}
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    </AuthProvider>
  );
}

export default App;
