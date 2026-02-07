import { NavLink } from "react-router-dom";
import { useAuth } from "../../store/AuthContext";

interface SidebarProps {
  isOpen: boolean;
  onClose: () => void;
}

const tenantNavItems = [
  { to: "/dashboard", label: "Home", icon: HomeIcon },
  { to: "/clients", label: "Clients", icon: ClientsIcon },
  { to: "/memberships", label: "Memberships", icon: MembershipsIcon },
  { to: "/bonuses", label: "Bonuses", icon: BonusesIcon },
  { to: "/payments", label: "Payments", icon: PaymentsIcon },
  { to: "/reports", label: "Reports", icon: ReportsIcon },
  { to: "/staff", label: "Staff", icon: StaffIcon },
  { to: "/settings", label: "Settings", icon: SettingsIcon },
];

const platformNavItems = [
  { to: "/platform", label: "Dashboard", icon: HomeIcon },
  { to: "/platform/tenants", label: "Tenants", icon: ClientsIcon },
  { to: "/platform/billing", label: "Billing", icon: PaymentsIcon },
];

export default function Sidebar({ isOpen, onClose }: SidebarProps) {
  const { user, isPlatformAdmin, logout } = useAuth();
  const navItems = isPlatformAdmin ? platformNavItems : tenantNavItems;

  return (
    <>
      {isOpen && (
        <div className="fixed inset-0 z-40 bg-black/30 lg:hidden" onClick={onClose} />
      )}

      <aside
        className={`fixed inset-y-0 left-0 z-50 flex w-[220px] flex-col bg-white border-r border-gray-200 transition-transform duration-200 ease-in-out lg:translate-x-0 lg:static lg:z-auto ${
          isOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        {/* Logo */}
        <div className="flex h-[60px] items-center gap-2.5 px-5 border-b border-gray-100">
          <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary-600 text-white text-xs font-bold">
            CR
          </div>
          <span className="text-[15px] font-semibold text-gray-900 tracking-tight">
            ClientRoll
          </span>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto py-3 px-2">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/dashboard" || item.to === "/platform"}
              onClick={onClose}
              className={({ isActive }) =>
                `group relative flex items-center gap-2.5 rounded-md px-3 py-[9px] text-[13px] font-medium transition-colors mb-0.5 ${
                  isActive
                    ? "bg-primary-50/80 text-primary-700"
                    : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
                }`
              }
            >
              {({ isActive }) => (
                <>
                  {isActive && (
                    <div className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-5 rounded-r-full bg-primary-600" />
                  )}
                  <item.icon active={isActive} />
                  {item.label}
                </>
              )}
            </NavLink>
          ))}
        </nav>

        {/* Refer & earn style button */}
        <div className="px-3 pb-2">
          <button className="flex w-full items-center gap-2 rounded-lg bg-primary-50 px-3 py-2.5 text-xs font-medium text-primary-700 hover:bg-primary-100 transition-colors">
            <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" /></svg>
            Share signup link
          </button>
        </div>

        {/* User */}
        <div className="border-t border-gray-100 px-3 py-3">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-gray-100 text-gray-600 text-xs font-semibold">
              {user?.first_name?.[0] || user?.name?.[0] || user?.email?.[0]?.toUpperCase() || "U"}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-[13px] font-medium text-gray-900 leading-tight">
                {user?.first_name ? `${user.first_name} ${user.last_name || ""}`.trim() : user?.name || user?.email}
              </p>
              <p className="truncate text-[11px] text-gray-400 leading-tight mt-0.5">
                {user?.role === "platform_admin" ? "Platform Admin" : user?.role === "tenant_admin" ? "Admin" : "Staff"}
              </p>
            </div>
            <button
              onClick={() => logout()}
              className="rounded p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors"
              title="Log out"
            >
              <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" /></svg>
            </button>
          </div>
        </div>
      </aside>
    </>
  );
}

// SVG Icons matching Ignition style (outline, 18px)
function HomeIcon({ active }: { active: boolean }) {
  return (
    <svg className={`h-[18px] w-[18px] ${active ? "text-primary-600" : "text-gray-400 group-hover:text-gray-500"}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
    </svg>
  );
}

function ClientsIcon({ active }: { active: boolean }) {
  return (
    <svg className={`h-[18px] w-[18px] ${active ? "text-primary-600" : "text-gray-400 group-hover:text-gray-500"}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
    </svg>
  );
}

function MembershipsIcon({ active }: { active: boolean }) {
  return (
    <svg className={`h-[18px] w-[18px] ${active ? "text-primary-600" : "text-gray-400 group-hover:text-gray-500"}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
    </svg>
  );
}

function BonusesIcon({ active }: { active: boolean }) {
  return (
    <svg className={`h-[18px] w-[18px] ${active ? "text-primary-600" : "text-gray-400 group-hover:text-gray-500"}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 8v13m0-13V6a2 2 0 112 2h-2zm0 0V5.5A2.5 2.5 0 109.5 8H12zm-7 4h14M5 12a2 2 0 110-4h14a2 2 0 110 4M5 12v7a2 2 0 002 2h10a2 2 0 002-2v-7" />
    </svg>
  );
}

function PaymentsIcon({ active }: { active: boolean }) {
  return (
    <svg className={`h-[18px] w-[18px] ${active ? "text-primary-600" : "text-gray-400 group-hover:text-gray-500"}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z" />
    </svg>
  );
}

function ReportsIcon({ active }: { active: boolean }) {
  return (
    <svg className={`h-[18px] w-[18px] ${active ? "text-primary-600" : "text-gray-400 group-hover:text-gray-500"}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
    </svg>
  );
}

function StaffIcon({ active }: { active: boolean }) {
  return (
    <svg className={`h-[18px] w-[18px] ${active ? "text-primary-600" : "text-gray-400 group-hover:text-gray-500"}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
    </svg>
  );
}

function SettingsIcon({ active }: { active: boolean }) {
  return (
    <svg className={`h-[18px] w-[18px] ${active ? "text-primary-600" : "text-gray-400 group-hover:text-gray-500"}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  );
}
