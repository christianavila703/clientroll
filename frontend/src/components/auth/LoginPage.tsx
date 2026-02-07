import { useState } from "react";
import { useNavigate, Link } from "react-router-dom";
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
      if (isPlatform) { await platformLogin(email, password); navigate("/platform"); }
      else { await login(email, password); navigate("/dashboard"); }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "An unexpected error occurred");
    } finally { setLoading(false); }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-sm">
        {/* Brand */}
        <div className="mb-8 flex flex-col items-center">
          <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-indigo-600 text-white text-sm font-bold shadow-sm">C</div>
          <p className="text-sm text-gray-500">{isPlatform ? "Platform admin sign in" : "Sign in to ClientRoll"}</p>
        </div>

        {/* Form card */}
        <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
          {error && (
            <div className="mb-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="email" className="mb-1 block text-[13px] font-medium text-gray-700">Email</label>
              <input id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} className="input-field" placeholder="you@company.com" required autoComplete="email" />
            </div>

            <div>
              <label htmlFor="password" className="mb-1 block text-[13px] font-medium text-gray-700">Password</label>
              <div className="relative">
                <input id="password" type={showPassword ? "text" : "password"} value={password} onChange={(e) => setPassword(e.target.value)} className="input-field pr-14" placeholder="Enter password" required autoComplete="current-password" />
                <button type="button" onClick={() => setShowPassword(!showPassword)} className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[11px] font-medium text-gray-500 hover:text-gray-700">{showPassword ? "Hide" : "Show"}</button>
              </div>
            </div>

            <button type="submit" disabled={loading} className="btn-primary btn-lg w-full justify-center">
              {loading ? "Signing in..." : "Sign in"}
            </button>
          </form>

          <div className="mt-4 text-center">
            <Link to="/forgot-password" className="text-[13px] text-indigo-600 hover:text-indigo-700">Forgot password?</Link>
          </div>
        </div>

        <p className="mt-4 text-center text-[13px] text-gray-500">
          Don't have an account? <Link to="/register" className="text-indigo-600 hover:text-indigo-700 font-medium">Create one</Link>
        </p>

        <div className="mt-2 text-center">
          <button onClick={() => setIsPlatform(!isPlatform)} className="text-[11px] text-gray-400 hover:text-gray-600 transition-colors">
            {isPlatform ? "← Tenant login" : "Platform admin →"}
          </button>
        </div>
      </div>
    </div>
  );
}
