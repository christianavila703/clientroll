import { useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useAuth } from "../../store/AuthContext";
import { ApiError } from "../../api/client";
import { brand } from "../../assets/brand";

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

  const features = [
    {
      icon: (
        <div className="flex h-14 w-14 items-center justify-center rounded-xl bg-indigo-50">
          <svg className="h-7 w-7 text-indigo-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
        </div>
      ),
      title: "Membership Management",
      desc: "Create, manage and track client memberships with flexible terms and pricing.",
    },
    {
      icon: (
        <div className="flex h-14 w-14 items-center justify-center rounded-xl bg-emerald-50">
          <svg className="h-7 w-7 text-emerald-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" /></svg>
        </div>
      ),
      title: "Direct Debit Payments",
      desc: "Automated payment collection with seamless direct debit integration.",
    },
    {
      icon: (
        <div className="flex h-14 w-14 items-center justify-center rounded-xl bg-amber-50">
          <svg className="h-7 w-7 text-amber-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" /></svg>
        </div>
      ),
      title: "Financial Reports",
      desc: "Real-time dashboards, revenue tracking, and payment analytics.",
    },
    {
      icon: (
        <div className="flex h-14 w-14 items-center justify-center rounded-xl bg-rose-50">
          <svg className="h-7 w-7 text-rose-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z" /></svg>
        </div>
      ),
      title: "Client Signup Flow",
      desc: "Beautiful public signup pages with digital signatures on any device.",
    },
  ];

  return (
    <div className="flex min-h-screen">
      {/* Left side — form */}
      <div className="flex flex-1 flex-col items-center justify-center px-6 py-12">
        <div className="w-full max-w-sm">
          {/* Logo */}
          <div className="mb-10">
            <div className="mb-8">
              <img src={brand.logo} alt={brand.appName} className="h-9 w-auto object-contain" />
            </div>
            <h1 className="text-[32px] font-bold text-gray-900 leading-tight tracking-tight">
              Log in to your account
            </h1>
          </div>

          {error && (
            <div className="mb-5 rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-[13px] text-red-600">{error}</div>
          )}

          <form onSubmit={handleSubmit} className="space-y-7">
            <div>
              <label htmlFor="email" className="block text-[15px] font-medium text-gray-900 mb-2">
                Business email
              </label>
              <input
                id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)}
                className="w-full rounded-lg border border-gray-300 bg-white px-4 py-3 text-[15px] text-gray-900 placeholder:text-gray-400 focus:border-indigo-600 focus:ring-2 focus:ring-indigo-600 focus:outline-none transition-all"
                placeholder="you@company.com" required autoComplete="email"
              />
            </div>

            <div>
              <div className="flex items-center justify-between mb-2">
                <label htmlFor="password" className="block text-[15px] font-medium text-gray-900">Password</label>
                <Link to="/forgot-password" className="text-[14px] text-indigo-600 hover:text-indigo-700 font-semibold">Forgot password?</Link>
              </div>
              <div className="relative">
                <input
                  id="password" type={showPassword ? "text" : "password"} value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full rounded-lg border border-gray-300 bg-white px-4 py-3 pr-14 text-[15px] text-gray-900 placeholder:text-gray-400 focus:border-indigo-600 focus:ring-2 focus:ring-indigo-600 focus:outline-none transition-all"
                  required autoComplete="current-password"
                />
                <button type="button" onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3.5 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 p-1">
                  {showPassword ? (
                    <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M3.98 8.223A10.477 10.477 0 001.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.45 10.45 0 0112 4.5c4.756 0 8.773 3.162 10.065 7.498a10.523 10.523 0 01-4.293 5.774M6.228 6.228L3 3m3.228 3.228l3.65 3.65m7.894 7.894L21 21m-3.228-3.228l-3.65-3.65m0 0a3 3 0 10-4.243-4.243m4.242 4.242L9.88 9.88" /></svg>
                  ) : (
                    <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" /><path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
                  )}
                </button>
              </div>
            </div>

            <button type="submit" disabled={loading}
              className="w-full rounded-lg bg-indigo-600 px-4 py-3 text-[15px] font-semibold text-white hover:bg-indigo-700 focus:outline-none disabled:opacity-50 transition-colors"
            >
              {loading ? "Logging in..." : "Log in"}
            </button>

            <Link to="/register"
              className="flex w-full items-center justify-center rounded-lg border border-gray-300 bg-white px-4 py-3 text-[15px] font-medium text-indigo-600 hover:bg-gray-50 transition-colors"
            >
              New to ClientRoll? Sign up
            </Link>
          </form>

          <div className="mt-6 text-center">
            <button onClick={() => setIsPlatform(!isPlatform)} className="text-[11px] text-gray-400 hover:text-gray-600 transition-colors">
              {isPlatform ? "← Tenant login" : "Platform admin →"}
            </button>
          </div>
        </div>
      </div>

      {/* Right side — features (hidden on mobile) */}
      <div className="hidden lg:flex lg:flex-1 items-center justify-center bg-gray-50 border-l border-gray-200 px-12 xl:px-20">
        <div className="max-w-md space-y-8">
          {features.map((f, i) => (
            <div key={i} className="flex items-start gap-4">
              {f.icon}
              <div>
                <h3 className="text-[15px] font-semibold text-gray-900">{f.title}</h3>
                <p className="text-[13px] text-gray-500 mt-0.5 leading-relaxed">{f.desc}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
