import { useState, useEffect } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { brand } from "../../assets/brand";

export default function VerifyEmailPage() {
  const [params] = useSearchParams();
  const token = params.get("token");
  const [status, setStatus] = useState<"loading" | "success" | "error">("loading");
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (!token) {
      setStatus("error");
      setMessage("No verification token provided.");
      return;
    }
    verify();
  }, [token]);

  const verify = async () => {
    try {
      const res = await fetch(`/api/auth/verify-email?token=${token}`);
      const data = await res.json();
      if (res.ok && data.verified) {
        setStatus("success");
        setMessage(data.message || "Email verified successfully.");
      } else {
        setStatus("error");
        setMessage(data.error || "Verification failed.");
      }
    } catch {
      setStatus("error");
      setMessage("Verification failed. Please try again.");
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center">
          <img src={brand.logo} alt={brand.appName} className="mb-4 h-9 w-auto object-contain" />
        </div>

        <div className="rounded-lg border border-gray-200 bg-white px-6 py-6 shadow-sm text-center">
          {status === "loading" && (
            <>
              <div className="mx-auto mb-4 h-5 w-5 animate-spin rounded-full border-2 border-gray-200 border-t-indigo-600" />
              <p className="text-[13px] text-gray-500">Verifying your email...</p>
            </>
          )}

          {status === "success" && (
            <>
              <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-emerald-50">
                <svg className="h-6 w-6 text-emerald-600" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
              </div>
              <h2 className="text-base font-semibold text-gray-900 mb-1">Email verified</h2>
              <p className="text-[13px] text-gray-500 mb-4">{message}</p>
              <Link to="/login" className="btn-primary btn-lg justify-center inline-flex">Sign in</Link>
            </>
          )}

          {status === "error" && (
            <>
              <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-red-50">
                <svg className="h-6 w-6 text-red-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
              </div>
              <h2 className="text-base font-semibold text-gray-900 mb-1">Verification failed</h2>
              <p className="text-[13px] text-gray-500 mb-4">{message}</p>
              <ResendForm />
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function ResendForm() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleResend = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await fetch("/api/auth/resend-verification", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
    } catch { /* always show success */ }
    setSent(true);
    setLoading(false);
  };

  if (sent) {
    return <p className="text-[13px] text-emerald-600">If an unverified account exists, a new link has been sent.</p>;
  }

  return (
    <form onSubmit={handleResend} className="mt-4 space-y-3">
      <p className="text-[12px] text-gray-400">Request a new verification link:</p>
      <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@company.com" className="input-field" required />
      <button type="submit" disabled={loading} className="btn-secondary btn-lg w-full justify-center">
        {loading ? "Sending..." : "Resend verification"}
      </button>
    </form>
  );
}
