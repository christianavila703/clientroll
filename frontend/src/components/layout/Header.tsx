import { useState, useRef, useEffect } from "react";
import { useNavigate } from "react-router-dom";

interface HeaderProps {
  onMenuToggle: () => void;
}

interface SearchResult {
  type: "client" | "membership" | "bonus";
  id: string;
  title: string;
  subtitle?: string;
  url: string;
}

export default function Header({ onMenuToggle }: HeaderProps) {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const search = (q: string) => {
    setQuery(q);
    if (q.length < 2) { setResults([]); setOpen(false); return; }

    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(async () => {
      setLoading(true);
      const token = localStorage.getItem("access_token");
      const headers = { Authorization: `Bearer ${token}` };
      const items: SearchResult[] = [];

      try {
        const [clientsRes, mtRes, bonusRes] = await Promise.all([
          fetch(`/api/tenant/clients?search=${encodeURIComponent(q)}`, { headers }).then(r => r.json()),
          fetch("/api/tenant/membership-types", { headers }).then(r => r.json()),
          fetch("/api/tenant/bonuses", { headers }).then(r => r.json()),
        ]);

        (clientsRes.clients || []).slice(0, 5).forEach((c: { id: string; name: string; email?: string }) => {
          if (c.name.toLowerCase().includes(q.toLowerCase()) || c.email?.toLowerCase().includes(q.toLowerCase())) {
            items.push({ type: "client", id: c.id, title: c.name, subtitle: c.email, url: "/clients" });
          }
        });

        (mtRes.membership_types || []).slice(0, 5).forEach((mt: { id: string; title: string; slug: string }) => {
          if (mt.title.toLowerCase().includes(q.toLowerCase())) {
            items.push({ type: "membership", id: mt.id, title: mt.title, subtitle: `/${mt.slug}`, url: `/memberships/${mt.id}` });
          }
        });

        (bonusRes.bonuses || []).slice(0, 3).forEach((b: { id: string; title: string; value: number }) => {
          if (b.title.toLowerCase().includes(q.toLowerCase())) {
            items.push({ type: "bonus", id: b.id, title: b.title, subtitle: `$${b.value}`, url: "/bonuses" });
          }
        });

        setResults(items);
        setOpen(items.length > 0);
      } catch { /* handle */ }
      finally { setLoading(false); }
    }, 300);
  };

  const icons: Record<string, string> = {
    client: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z",
    membership: "M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z",
    bonus: "M12 8v13m0-13V6a2 2 0 112 2h-2zm0 0V5.5A2.5 2.5 0 109.5 8H12zm-7 4h14M5 12a2 2 0 110-4h14a2 2 0 110 4M5 12v7a2 2 0 002 2h10a2 2 0 002-2v-7",
  };

  return (
    <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-gray-200 bg-white px-4 lg:px-5">
      {/* Mobile menu */}
      <button onClick={onMenuToggle} className="rounded-md p-1.5 text-gray-500 hover:bg-gray-100 lg:hidden" aria-label="Menu">
        <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" /></svg>
      </button>

      {/* Search */}
      <div className="hidden md:block relative flex-1 max-w-sm" ref={ref}>
        <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
        <input
          type="text"
          value={query}
          onChange={(e) => search(e.target.value)}
          onFocus={() => { if (results.length > 0) setOpen(true); }}
          placeholder="Search clients, memberships..."
          className="w-full rounded-md border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-[13px] text-gray-600 placeholder:text-gray-400 focus:border-gray-300 focus:bg-white focus:outline-none transition-colors"
        />
        {loading && <div className="absolute right-2.5 top-1/2 -translate-y-1/2 h-3 w-3 animate-spin rounded-full border border-gray-300 border-t-gray-600" />}

        {/* Results dropdown */}
        {open && (
          <div className="absolute left-0 right-0 top-full mt-1 rounded-lg border border-gray-200 bg-white py-1 shadow-lg z-50 max-h-80 overflow-y-auto">
            {results.map((r) => (
              <button
                key={`${r.type}-${r.id}`}
                onClick={() => { setOpen(false); setQuery(""); navigate(r.url); }}
                className="flex w-full items-center gap-3 px-3 py-2 text-left hover:bg-gray-50 transition-colors"
              >
                <svg className="h-4 w-4 text-gray-400 flex-shrink-0" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d={icons[r.type]} /></svg>
                <div className="min-w-0 flex-1">
                  <p className="text-[13px] font-medium text-gray-900 truncate">{r.title}</p>
                  {r.subtitle && <p className="text-[11px] text-gray-400 truncate">{r.subtitle}</p>}
                </div>
                <span className="text-[10px] text-gray-300 uppercase flex-shrink-0">{r.type}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="flex-1 md:hidden" />

      {/* Right */}
      <div className="flex items-center gap-1">
        <button className="rounded-md p-1.5 text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors" title="Notifications">
          <svg className="h-[18px] w-[18px]" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M14.857 17.082a23.848 23.848 0 005.454-1.31A8.967 8.967 0 0118 9.75V9A6 6 0 006 9v.75a8.967 8.967 0 01-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 01-5.714 0m5.714 0a3 3 0 11-5.714 0" /></svg>
        </button>
      </div>
    </header>
  );
}
