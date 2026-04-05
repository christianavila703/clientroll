import { useState, useRef, useEffect } from "react";
import { NavLink, useNavigate } from "react-router-dom";
import { useAuth } from "../../store/AuthContext";
import { brand } from "../../assets/brand";

interface SidebarProps {
  isOpen: boolean;
  onClose: () => void;
}

/* Navigation grouped like Ignition */
const mainNav = [
  { to: "/dashboard", label: "Dashboard", icon: "M2.25 12l8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" },
  { to: "/signup", label: "Client Signup", icon: "M15.59 14.37a6 6 0 01-5.84 7.38v-4.8m5.84-2.58a14.98 14.98 0 006.16-12.12A14.98 14.98 0 009.631 8.41m5.96 5.96a14.926 14.926 0 01-5.841 2.58m-.119-8.54a6 6 0 00-7.381 5.84h4.8m2.581-5.84a14.927 14.927 0 00-2.58 5.84m2.699 2.7c-.103.021-.207.041-.311.06a15.09 15.09 0 01-2.448-2.448 14.9 14.9 0 01.06-.312m-2.24 2.39a4.493 4.493 0 00-1.757 4.306 4.493 4.493 0 004.306-1.758M16.5 9a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0z" },
];

const managementNav = [
  { to: "/clients", label: "Clients", icon: "M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" },
  { to: "/active-memberships", label: "Active Members", icon: "M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 002.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 00-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 00.75-.75 2.25 2.25 0 00-.1-.664m-5.8 0A2.251 2.251 0 0113.5 2.25H15c1.012 0 1.867.668 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25zM6.75 12h.008v.008H6.75V12zm0 3h.008v.008H6.75V15zm0 3h.008v.008H6.75V18z" },
  { to: "/memberships", label: "Membership Types", icon: "M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z" },
  { to: "/bonuses", label: "Bonuses", icon: "M21 11.25v8.25a1.5 1.5 0 01-1.5 1.5H5.25a1.5 1.5 0 01-1.5-1.5v-8.25M12 4.875A2.625 2.625 0 109.375 7.5H12m0-2.625V7.5m0-2.625A2.625 2.625 0 1114.625 7.5H12m0 0V21m-8.625-9.75h18c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125h-18c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" },
  { to: "/services", label: "Services", icon: "M11.42 15.17L17.25 21A2.652 2.652 0 0021 17.25l-5.877-5.877M11.42 15.17l2.496-3.03c.317-.384.74-.626 1.208-.766M11.42 15.17l-4.655 5.653a2.548 2.548 0 11-3.586-3.586l5.653-4.657m5.014-.134a4.486 4.486 0 00-1.753-4.478 4.486 4.486 0 00-6.236.61l-1.006 1.006" },
  { to: "/terms", label: "Terms", icon: "M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" },
];

const financeNav = [
  { to: "/payments", label: "Payments", icon: "M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z" },
  { to: "/reports", label: "Reports", icon: "M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" },
  { to: "/payment-monitoring", label: "Monitoring", icon: "M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" },
];

const adminNav = [
  { to: "/staff", label: "Team", icon: "M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z" },
  { to: "/email-templates", label: "Templates", icon: "M21.75 6.75v10.5a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25m19.5 0v.243a2.25 2.25 0 01-1.07 1.916l-7.5 4.615a2.25 2.25 0 01-2.36 0L3.32 8.91a2.25 2.25 0 01-1.07-1.916V6.75" },
  { to: "/settings", label: "Settings", icon: "M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z M15 12a3 3 0 11-6 0 3 3 0 016 0z" },
];

const platformNavItems = [
  { to: "/platform", label: "Overview", icon: "M2.25 12l8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" },
  { to: "/platform/tenants", label: "Tenants", icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" },
  { to: "/platform/billing", label: "Billing", icon: "M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" },
];

const createActions = [
  { label: "Client", desc: "Add to your client list", to: "/clients?new=1", icon: "M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z" },
  { label: "Membership Type", desc: "Define a new offering", to: "/memberships?new=1", icon: "M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z" },
  { label: "Bonus", desc: "Create a signup bonus", to: "/bonuses?new=1", icon: "M21 11.25v8.25a1.5 1.5 0 01-1.5 1.5H5.25a1.5 1.5 0 01-1.5-1.5v-8.25M12 4.875A2.625 2.625 0 109.375 7.5H12m0-2.625V7.5m0-2.625A2.625 2.625 0 1114.625 7.5H12m0 0V21m-8.625-9.75h18c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125h-18c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" },
];

export default function Sidebar({ isOpen, onClose }: SidebarProps) {
  const { user, isPlatformAdmin, logout } = useAuth();
  const navigate = useNavigate();
  const [createOpen, setCreateOpen] = useState(false);
  const createRef = useRef<HTMLDivElement>(null);
  const buildSha = typeof __APP_BUILD_SHA__ !== "undefined" ? __APP_BUILD_SHA__ : "";

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (createRef.current && !createRef.current.contains(e.target as Node)) setCreateOpen(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const NavItem = ({ to, label, icon, end }: { to: string; label: string; icon: string; end?: boolean }) => (
    <NavLink
      to={to}
      end={end}
      onClick={onClose}
      className={({ isActive }) =>
        `group flex items-center gap-2.5 rounded-lg px-3 py-2 text-[13px] font-medium transition-all ${
          isActive
            ? "bg-indigo-50 text-indigo-700"
            : "text-gray-600 hover:text-gray-900 hover:bg-gray-50"
        }`
      }
    >
      {({ isActive }) => (
        <>
          <svg className={`h-[18px] w-[18px] flex-shrink-0 transition-colors ${isActive ? "text-indigo-600" : "text-gray-400 group-hover:text-gray-500"}`} fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d={icon} />
          </svg>
          {label}
        </>
      )}
    </NavLink>
  );

  const SectionLabel = ({ label }: { label: string }) => (
    <div className="px-3 pt-5 pb-1">
      <p className="text-[10px] font-semibold uppercase tracking-widest text-gray-400">{label}</p>
    </div>
  );

  return (
    <>
      {isOpen && <div className="fixed inset-0 z-40 bg-gray-900/30 backdrop-blur-sm lg:hidden" onClick={onClose} />}

      <aside className={`fixed inset-y-0 left-0 z-50 flex w-[220px] flex-col bg-white border-r border-gray-200 transition-transform duration-200 lg:translate-x-0 lg:static lg:z-auto ${isOpen ? "translate-x-0" : "-translate-x-full"}`}>
        {/* Brand */}
        <div className="flex h-14 items-center gap-2.5 px-4 border-b border-gray-100">
          <img src={brand.logo} alt="" className="h-8 w-auto max-w-[140px] object-contain" />
        </div>

        {/* Create */}
        {!isPlatformAdmin && (
          <div className="px-3 pt-4 pb-1" ref={createRef}>
            <div className="relative">
              <button
                onClick={() => setCreateOpen(!createOpen)}
                className="flex w-full items-center justify-center gap-2 rounded-lg border border-indigo-200 bg-gradient-to-r from-indigo-50 to-violet-50 px-3 py-2.5 text-[13px] font-semibold text-indigo-700 hover:from-indigo-100 hover:to-violet-100 transition-all shadow-sm"
              >
                <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" /></svg>
                Create
                <svg className={`h-3 w-3 ml-auto transition-transform ${createOpen ? "rotate-180" : ""}`} fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" /></svg>
              </button>

              {createOpen && (
                <div className="absolute left-0 right-0 top-full mt-1.5 rounded-xl border border-gray-200 bg-white py-1.5 shadow-xl z-50">
                  {createActions.map((action) => (
                    <button
                      key={action.to}
                      onClick={() => { setCreateOpen(false); onClose(); navigate(action.to); }}
                      className="flex w-full items-center gap-3 px-3 py-2.5 text-left hover:bg-gray-50 transition-colors"
                    >
                      <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100">
                        <svg className="h-4 w-4 text-gray-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d={action.icon} /></svg>
                      </div>
                      <div>
                        <p className="text-[13px] font-medium text-gray-900">{action.label}</p>
                        <p className="text-[11px] text-gray-400">{action.desc}</p>
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto px-2.5 pb-2">
          {isPlatformAdmin ? (
            <div className="space-y-0.5 pt-2">
              {platformNavItems.map((item) => (
                <NavItem key={item.to} {...item} end={item.to === "/platform"} />
              ))}
            </div>
          ) : (
            <>
              {/* Main */}
              <div className="space-y-0.5 pt-2">
                {mainNav.map((item) => (
                  <NavItem key={item.to} {...item} end={item.to === "/dashboard"} />
                ))}
              </div>

              {/* Management */}
              <SectionLabel label="Management" />
              <div className="space-y-0.5">
                {managementNav.map((item) => (
                  <NavItem key={item.to} {...item} />
                ))}
              </div>

              {/* Finance */}
              <SectionLabel label="Finance" />
              <div className="space-y-0.5">
                {financeNav.map((item) => (
                  <NavItem key={item.to} {...item} />
                ))}
              </div>

              {/* Admin */}
              <SectionLabel label="Admin" />
              <div className="space-y-0.5">
                {adminNav.map((item) => (
                  <NavItem key={item.to} {...item} />
                ))}
              </div>
            </>
          )}
        </nav>

        {/* User footer */}
        <div className="border-t border-gray-100 px-3 py-3">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500 to-violet-500 text-[11px] font-bold text-white shadow-sm flex-shrink-0">
              {(user?.first_name?.[0] || user?.email?.[0] || "U").toUpperCase()}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-[13px] font-semibold text-gray-900 leading-tight">
                {user?.first_name ? `${user.first_name} ${user.last_name || ""}`.trim() : user?.email || "User"}
              </p>
              <p className="truncate text-[11px] text-gray-400 leading-tight">
                {user?.role === "platform_admin" ? "Platform Admin" : user?.role === "tenant_admin" ? "Admin" : "Staff"}
              </p>
              {buildSha && (
                <p className="truncate text-[10px] text-gray-300 leading-tight mt-0.5">
                  Build {buildSha}
                </p>
              )}
            </div>
            <button onClick={() => logout()} className="rounded-lg p-1.5 text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors" title="Sign out">
              <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15m3 0l3-3m0 0l-3-3m3 3H9" /></svg>
            </button>
          </div>
        </div>
      </aside>
    </>
  );
}
