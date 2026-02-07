import { useState, useCallback } from "react";
import { useNavigate, Link } from "react-router-dom";
import { ApiError } from "../../api/client";

export default function RegisterPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<1 | 2>(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const [company, setCompany] = useState({ company_name: "", slug: "", timezone: "Australia/Brisbane" });
  const [admin, setAdmin] = useState({ first_name: "", last_name: "", email: "", password: "" });
  const [slugStatus, setSlugStatus] = useState<"idle" | "checking" | "available" | "taken">("idle");

  const checkSlug = useCallback(async (slug: string) => {
    if (slug.length < 3) { setSlugStatus("idle"); return; }
    setSlugStatus("checking");
    try {
      const res = await fetch(`/api/register/check-slug/${slug.toLowerCase()}`);
      const data = await res.json();
      setSlugStatus(data.available ? "available" : "taken");
    } catch { setSlugStatus("idle"); }
  }, []);

  const handleCompanyNameChange = (name: string) => {
    setCompany(prev => {
      const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
      checkSlug(slug);
      return { ...prev, company_name: name, slug };
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const res = await fetch("/api/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...company, ...admin }),
      });
      const data = await res.json();
      if (!res.ok) throw new ApiError(data.error || "Registration failed", res.status);

      // Auto-login
      localStorage.setItem("access_token", data.tokens.access_token);
      localStorage.setItem("refresh_token", data.tokens.refresh_token);
      navigate("/dashboard");
      window.location.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Registration failed");
    } finally {
      setLoading(false);
    }
  };

  const timezones = [
    "Australia/Brisbane", "Australia/Sydney", "Australia/Melbourne", "Australia/Perth",
    "Australia/Adelaide", "Australia/Darwin", "Australia/Hobart",
    "Pacific/Auckland", "America/New_York", "America/Chicago", "America/Los_Angeles",
    "Europe/London", "Europe/Berlin", "Asia/Singapore", "Asia/Tokyo",
  ];

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 px-4 py-12">
      <div className="w-full max-w-md">
        {/* Brand */}
        <div className="mb-8 flex flex-col items-center">
          <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-indigo-600 text-white text-sm font-bold shadow-sm">C</div>
          <h1 className="text-lg font-semibold text-gray-900">Create your account</h1>
          <p className="mt-1 text-[13px] text-gray-500">Start managing memberships in minutes</p>
        </div>

        <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
          {error && (
            <div className="mb-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>
          )}

          {/* Progress */}
          <div className="mb-6 flex items-center gap-3">
            <div className={`flex h-6 w-6 items-center justify-center rounded-full text-[11px] font-semibold ${step >= 1 ? "bg-indigo-600 text-white" : "bg-gray-100 text-gray-400"}`}>1</div>
            <div className={`h-px flex-1 ${step >= 2 ? "bg-indigo-600" : "bg-gray-200"}`} />
            <div className={`flex h-6 w-6 items-center justify-center rounded-full text-[11px] font-semibold ${step >= 2 ? "bg-indigo-600 text-white" : "bg-gray-100 text-gray-400"}`}>2</div>
          </div>

          <form onSubmit={handleSubmit}>
            {step === 1 && (
              <div className="space-y-4">
                <p className="text-[13px] font-medium text-gray-900 mb-3">Company details</p>

                <div>
                  <label className="mb-1 block text-[13px] font-medium text-gray-700">Company name</label>
                  <input
                    value={company.company_name}
                    onChange={(e) => handleCompanyNameChange(e.target.value)}
                    className="input-field"
                    placeholder="Acme Wellness"
                    required
                  />
                </div>

                <div>
                  <label className="mb-1 block text-[13px] font-medium text-gray-700">URL</label>
                  <div className="flex items-center gap-0">
                    <span className="rounded-l-md border border-r-0 border-gray-300 bg-gray-50 px-3 py-[7px] text-[13px] text-gray-500">clientroll.com/</span>
                    <input
                      value={company.slug}
                      onChange={(e) => { setCompany(prev => ({ ...prev, slug: e.target.value.toLowerCase() })); checkSlug(e.target.value); }}
                      className="input-field rounded-l-none flex-1"
                      placeholder="acme-wellness"
                      required
                    />
                  </div>
                  {slugStatus === "checking" && <p className="mt-1 text-[11px] text-gray-400">Checking availability...</p>}
                  {slugStatus === "available" && <p className="mt-1 text-[11px] text-emerald-600">Available</p>}
                  {slugStatus === "taken" && <p className="mt-1 text-[11px] text-red-600">Already taken — try another</p>}
                </div>

                <div>
                  <label className="mb-1 block text-[13px] font-medium text-gray-700">Timezone</label>
                  <select
                    value={company.timezone}
                    onChange={(e) => setCompany(prev => ({ ...prev, timezone: e.target.value }))}
                    className="input-field"
                  >
                    {timezones.map((tz) => <option key={tz} value={tz}>{tz.replace("_", " ")}</option>)}
                  </select>
                </div>

                <button
                  type="button"
                  onClick={() => { if (company.company_name && company.slug && slugStatus !== "taken") setStep(2); }}
                  disabled={!company.company_name || !company.slug || slugStatus === "taken"}
                  className="btn-primary btn-lg w-full justify-center mt-2"
                >
                  Continue
                </button>
              </div>
            )}

            {step === 2 && (
              <div className="space-y-4">
                <p className="text-[13px] font-medium text-gray-900 mb-3">Your details</p>

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="mb-1 block text-[13px] font-medium text-gray-700">First name</label>
                    <input value={admin.first_name} onChange={(e) => setAdmin(prev => ({ ...prev, first_name: e.target.value }))} className="input-field" required />
                  </div>
                  <div>
                    <label className="mb-1 block text-[13px] font-medium text-gray-700">Last name</label>
                    <input value={admin.last_name} onChange={(e) => setAdmin(prev => ({ ...prev, last_name: e.target.value }))} className="input-field" />
                  </div>
                </div>

                <div>
                  <label className="mb-1 block text-[13px] font-medium text-gray-700">Email</label>
                  <input type="email" value={admin.email} onChange={(e) => setAdmin(prev => ({ ...prev, email: e.target.value }))} className="input-field" placeholder="you@company.com" required />
                </div>

                <div>
                  <label className="mb-1 block text-[13px] font-medium text-gray-700">Password</label>
                  <input type="password" value={admin.password} onChange={(e) => setAdmin(prev => ({ ...prev, password: e.target.value }))} className="input-field" placeholder="Min 10 characters" required minLength={10} />
                  <p className="mt-1 text-[11px] text-gray-400">Must include uppercase, lowercase, and a number</p>
                </div>

                <div className="flex gap-3 mt-2">
                  <button type="button" onClick={() => setStep(1)} className="btn-secondary btn-lg flex-1 justify-center">Back</button>
                  <button type="submit" disabled={loading || !admin.email || !admin.password || !admin.first_name} className="btn-primary btn-lg flex-1 justify-center">
                    {loading ? "Creating..." : "Create account"}
                  </button>
                </div>
              </div>
            )}
          </form>
        </div>

        <p className="mt-4 text-center text-[13px] text-gray-500">
          Already have an account? <Link to="/login" className="text-indigo-600 hover:text-indigo-700 font-medium">Sign in</Link>
        </p>
      </div>
    </div>
  );
}
