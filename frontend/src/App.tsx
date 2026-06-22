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
import ClientProfilePage from "./components/clients/ClientProfilePage";
import MembershipTypesPage from "./components/memberships/MembershipTypesPage";
import MembershipTypeDetail from "./components/memberships/MembershipTypeDetail";
import ActiveMembershipsPage from "./components/memberships/ActiveMembershipsPage";
import SignupPage from "./components/signup/SignupPage";
import PaymentsPage from "./components/payments/PaymentsPage";
import ReportsPage from "./components/reports/ReportsPage";
import TermsPage from "./components/terms/TermsPage";
import EmailTemplatesPage from "./components/settings/EmailTemplatesPage";
import BonusesPage from "./components/bonuses/BonusesPage";
import StaffSignupLauncher from "./components/signup/StaffSignupLauncher";
import ServicesPage from "./components/services/ServicesPage";
import RequirePlatformAdmin from "./components/platform/RequirePlatformAdmin";
import PlatformTenantCreatePage from "./components/platform/PlatformTenantCreatePage";
import PlatformBillingPage from "./components/platform/PlatformBillingPage";
import PlatformTenantBillingDetailPage from "./components/platform/PlatformTenantBillingDetailPage";
import PaymentMonitoringPage from "./components/monitoring/PaymentMonitoringPage";
import ReacceptPage from "./components/reaccept/ReacceptPage";

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
        <Route path="/join/:tenantSlug/invite/:inviteToken" element={<SignupPage />} />
        <Route path="/join/:tenantSlug" element={<SignupPage />} />
        {/* Public T&C re-acceptance flow (tokenised, no login) */}
        <Route path="/reaccept/:tenantSlug/:token" element={<ReacceptPage />} />
        <Route path="/:tenantSlug/:membershipSlug" element={<SignupPage />} />

        {/* Authenticated tenant routes */}
        <Route element={<AppShell />}>
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/clients" element={<ClientsPage />} />
          <Route path="/clients/:clientId" element={<ClientProfilePage />} />
          <Route path="/active-memberships" element={<ActiveMembershipsPage />} />
          <Route path="/memberships" element={<MembershipTypesPage />} />
          <Route path="/memberships/:mtId" element={<MembershipTypeDetail />} />
          <Route path="/bonuses" element={<BonusesPage />} />
          <Route path="/services" element={<ServicesPage />} />
          <Route path="/terms" element={<TermsPage />} />
          <Route path="/payments" element={<PaymentsPage />} />
          <Route path="/reports" element={<ReportsPage />} />
          <Route path="/staff" element={<StaffPage />} />
          <Route path="/signup" element={<StaffSignupLauncher />} />
          <Route path="/email-templates" element={<EmailTemplatesPage />} />
          <Route path="/settings" element={<SettingsPage />} />

          {/* Platform admin routes */}
          <Route element={<RequirePlatformAdmin />}>
            <Route path="/platform" element={<PlatformDashboard />} />
            <Route path="/platform/tenants" element={<PlatformDashboard />} />
            <Route path="/platform/tenants/new" element={<PlatformTenantCreatePage />} />
            <Route path="/platform/billing" element={<PlatformBillingPage />} />
            <Route path="/platform/billing/tenants/:tenantId" element={<PlatformTenantBillingDetailPage />} />
            <Route path="/platform/monitoring" element={<PaymentMonitoringPage />} />
          </Route>
        </Route>

        {/* Default redirect */}
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </AuthProvider>
  );
}

export default App;
