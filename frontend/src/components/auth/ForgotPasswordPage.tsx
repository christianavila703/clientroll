import { useState } from "react";
import { Link } from "react-router-dom";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await fetch("/api/auth/forgot-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      setSent(true);
    } catch { /* always show success */ setSent(true); }
    finally { setLoading(false); }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center">
          <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-indigo-600 text-white text-sm font-bold shadow-sm">C</div>
          <p className="text-sm text-gray-500">Reset your password</p>
        </div>

        <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
          {sent ? (
            <div className="text-center">
              <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-emerald-50">
                <svg className="h-6 w-6 text-emerald-600" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25m19.5 0v.243a2.25 2.25 0 01-1.07 1.916l-7.5 4.615a2.25 2.25 0 01-2.36 0L3.32 8.91a2.25 2.25 0 01-1.07-1.916V6.75" /></svg>
              </div>
              <h2 className="text-base font-semibold text-gray-900 mb-1">Check your email</h2>
              <p className="text-[13px] text-gray-500 mb-4">If an account exists for <strong>{email}</strong>, we've sent a password reset link.</p>
              <Link to="/login" className="text-[13px] text-indigo-600 hover:text-indigo-700 font-medium">Back to sign in</Link>
            </div>
          ) : (
            <>
              <p className="text-[13px] text-gray-500 mb-4">Enter your email address and we'll send you a link to reset your password.</p>
              <form onSubmit={handleSubmit} className="space-y-4">
                <div>
                  <label className="mb-1 block text-[13px] font-medium text-gray-700">Email</label>
                  <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} className="input-field" placeholder="you@company.com" required />
                </div>
                <button type="submit" disabled={loading} className="btn-primary btn-lg w-full justify-center">
                  {loading ? "Sending..." : "Send reset link"}
                </button>
              </form>
              <div className="mt-4 text-center">
                <Link to="/login" className="text-[13px] text-gray-500 hover:text-gray-700">Back to sign in</Link>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
