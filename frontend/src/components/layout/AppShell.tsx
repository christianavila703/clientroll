import { useState, useEffect } from "react";
import { Outlet, Navigate } from "react-router-dom";
import { useAuth } from "../../store/AuthContext";
import Sidebar from "./Sidebar";
import Header from "./Header";
import SetupWizard from "../setup/SetupWizard";

export default function AppShell() {
  const { isAuthenticated, isLoading, needsSetup } = useAuth();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [showWizard, setShowWizard] = useState(false);

  useEffect(() => {
    if (needsSetup) {
      setShowWizard(true);
    }
  }, [needsSetup]);

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-5 w-5 animate-spin rounded-full border-2 border-gray-200 border-t-indigo-600" />
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return (
    <div className="flex min-h-screen">
      <Sidebar isOpen={sidebarOpen} onClose={() => setSidebarOpen(false)} />
      <div className="flex flex-1 flex-col min-w-0">
        <Header onMenuToggle={() => setSidebarOpen(!sidebarOpen)} />
        <main className="flex-1 bg-gray-50/50 px-5 py-5 lg:px-8 lg:py-6 min-h-[calc(100vh-3.5rem)]">
          <Outlet context={{ showWizard: needsSetup, openWizard: () => setShowWizard(true) }} />
        </main>
      </div>
      {showWizard && <SetupWizard onClose={() => setShowWizard(false)} />}
    </div>
  );
}
