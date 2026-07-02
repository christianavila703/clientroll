import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "../../store/AuthContext";

export default function RequirePlatformAdmin() {
  const { isLoading, isAuthenticated, isPlatformAdmin } = useAuth();

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
  if (!isPlatformAdmin) {
    return <Navigate to="/dashboard" replace />;
  }
  return <Outlet />;
}
