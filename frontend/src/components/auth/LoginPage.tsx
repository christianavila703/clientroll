import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../store/AuthContext";
import { ApiError } from "../../api/client";

export default function LoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [isPlatform, setIsPlatform] = useState(false);
  const { login, platformLogin } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      if (isPlatform) {
        await platformLogin(email, password);
        navigate("/platform");
      } else {
        await login(email, password);
        navigate("/dashboard");
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "An unexpected error occurred");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-[#f7f7f8] px-4">
      <div className="w-full max-w-[420px]">
        {/* Card */}
        <div className="rounded-2xl border border-gray-200 bg-white px-8 py-10 shadow-sm">
          {/* Logo */}
          <div className="mb-8 text-center">
            <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-xl bg-primary-600 text-white font-bold text-lg">
              CR
            </div>
            <h1 className="text-lg font-semibold text-primary-600">
              {isPlatform ? "Platform Admin" : "Sign in to your account"}
            </h1>
          </div>

          {error && (
            <div className="mb-5 rounded-lg bg-red-50 border border-red-100 px-4 py-3 text-sm text-red-600">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-5">
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-gray-700 mb-1.5">
                Email
              </label>
              <input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="input-field"
                placeholder="Enter your email address"
                required
                autoComplete="email"
              />
            </div>

            <div>
              <label htmlFor="password" className="block text-sm font-medium text-gray-700 mb-1.5">
                Password
              </label>
              <div className="relative">
                <input
                  id="password"
                  type={showPassword ? "text" : "password"}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="input-field pr-16"
                  placeholder="Enter password"
                  required
                  autoComplete="current-password"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-xs font-medium text-gray-500 hover:text-gray-700"
                >
                  {showPassword ? "Hide" : "Show"}
                </button>
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full rounded-lg bg-primary-600 px-4 py-3 text-sm font-semibold text-white shadow-sm transition-all hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:opacity-50"
            >
              {loading ? "Signing in..." : "Sign in"}
            </button>
          </form>

          {/* Forgot password */}
          <div className="mt-5 text-center">
            <a href="#" className="text-sm text-primary-600 hover:text-primary-700 font-medium">
              Forgot Password?
            </a>
          </div>
        </div>

        {/* Toggle */}
        <div className="mt-4 text-center">
          <button
            type="button"
            onClick={() => setIsPlatform(!isPlatform)}
            className="text-xs text-gray-400 hover:text-gray-600 transition-colors"
          >
            {isPlatform ? "← Back to tenant login" : "Platform admin login →"}
          </button>
        </div>

        {/* Footer links */}
        <div className="mt-6 flex items-center justify-center gap-4 text-xs text-gray-400">
          <span>Website</span>
          <span>Help</span>
        </div>
      </div>
    </div>
  );
}
