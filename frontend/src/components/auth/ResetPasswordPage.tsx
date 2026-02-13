import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { brand } from "../../assets/brand";

export default function ResetPasswordPage() {
  const [params] = useSearchParams();
  const token = params.get("token") || "";
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [done, setDone] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (password !== confirm) { setError("Passwords don't match"); return; }
    setError(""); setLoading(true);
    try {
      const res = await fetch("/api/auth/reset-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token, password }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error);
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to reset password");
    } finally { setLoading(false); }
  };

  if (!token) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
        <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm max-w-sm text-center">
          <p className="text-[13px] text-gray-500 mb-3">Invalid reset link.</p>
          <Link to="/login" className="text-[13px] text-indigo-600 font-medium">Back to sign in</Link>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center">
          <img src={brand.logo} alt={brand.appName} className="mb-4 h-9 w-auto object-contain" />
          <p className="text-sm text-gray-500">Set new password</p>
        </div>

        <div className="rounded-lg border border-gray-200 bg-white px-6 py-6 shadow-sm">
          {done ? (
            <div className="text-center">
              <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-emerald-50">
                <svg className="h-6 w-6 text-emerald-600" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
              </div>
              <h2 className="text-base font-semibold text-gray-900 mb-1">Password updated</h2>
              <p className="text-[13px] text-gray-500 mb-4">You can now sign in with your new password.</p>
              <Link to="/login" className="btn-primary btn-lg justify-center inline-flex">Sign in</Link>
            </div>
          ) : (
            <>
              {error && <div className="mb-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}
              <form onSubmit={handleSubmit} className="space-y-4">
                <div>
                  <label className="mb-1 block text-[13px] font-medium text-gray-700">New password</label>
                  <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} className="input-field" placeholder="Min 10 characters" required minLength={10} />
                  <p className="mt-1 text-[11px] text-gray-400">Must include uppercase, lowercase, and a number</p>
                </div>
                <div>
                  <label className="mb-1 block text-[13px] font-medium text-gray-700">Confirm password</label>
                  <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} className="input-field" required />
                </div>
                <button type="submit" disabled={loading} className="btn-primary btn-lg w-full justify-center">
                  {loading ? "Updating..." : "Update password"}
                </button>
              </form>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
