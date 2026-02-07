import { NavLink } from "react-router-dom";
import { useAuth } from "../../store/AuthContext";

interface SidebarProps {
  isOpen: boolean;
  onClose: () => void;
}

const tenantNavItems = [
  { to: "/dashboard", label: "Dashboard", icon: "📊" },
  { to: "/clients", label: "Clients", icon: "👥" },
  { to: "/memberships", label: "Memberships", icon: "📋" },
  { to: "/bonuses", label: "Bonuses", icon: "🎁" },
  { to: "/payments", label: "Payments", icon: "💳" },
  { to: "/reports", label: "Reports", icon: "📈" },
  { to: "/staff", label: "Staff", icon: "👤" },
  { to: "/settings", label: "Settings", icon: "⚙️" },
];

const platformNavItems = [
  { to: "/platform", label: "Dashboard", icon: "📊" },
  { to: "/platform/tenants", label: "Tenants", icon: "🏢" },
  { to: "/platform/billing", label: "Billing", icon: "💰" },
];

export default function Sidebar({ isOpen, onClose }: SidebarProps) {
  const { user, isPlatformAdmin } = useAuth();
  const navItems = isPlatformAdmin ? platformNavItems : tenantNavItems;

  return (
    <>
      {/* Backdrop for mobile */}
      {isOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 lg:hidden"
          onClick={onClose}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-64 transform bg-white border-r border-gray-200 transition-transform duration-200 ease-in-out lg:translate-x-0 lg:static lg:z-auto ${
          isOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        {/* Logo */}
        <div className="flex h-16 items-center gap-3 border-b border-gray-200 px-6">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-600 text-white font-bold text-sm">
            CR
          </div>
          <span className="text-lg font-semibold text-gray-900">
            ClientRoll
          </span>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto p-4">
          <ul className="space-y-1">
            {navItems.map((item) => (
              <li key={item.to}>
                <NavLink
                  to={item.to}
                  onClick={onClose}
                  className={({ isActive }) =>
                    `flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors min-h-[44px] ${
                      isActive
                        ? "bg-primary-50 text-primary-700"
                        : "text-gray-700 hover:bg-gray-100"
                    }`
                  }
                >
                  <span className="text-lg">{item.icon}</span>
                  {item.label}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>

        {/* User info at bottom */}
        <div className="border-t border-gray-200 p-4">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary-100 text-primary-700 text-sm font-medium">
              {user?.first_name?.[0] || user?.name?.[0] || user?.email?.[0] || "U"}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-gray-900">
                {user?.first_name
                  ? `${user.first_name} ${user.last_name || ""}`
                  : user?.name || user?.email}
              </p>
              <p className="truncate text-xs text-gray-500">
                {user?.role?.replace("_", " ")}
              </p>
            </div>
          </div>
        </div>
      </aside>
    </>
  );
}
