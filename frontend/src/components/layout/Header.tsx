import { useAuth } from "../../store/AuthContext";
import { useNavigate } from "react-router-dom";

interface HeaderProps {
  onMenuToggle: () => void;
}

export default function Header({ onMenuToggle }: HeaderProps) {
  const { logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = async () => {
    await logout();
    navigate("/login");
  };

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center gap-4 border-b border-gray-200 bg-white px-4 sm:px-6">
      {/* Mobile menu button */}
      <button
        onClick={onMenuToggle}
        className="flex h-10 w-10 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 lg:hidden"
        aria-label="Toggle menu"
      >
        <svg
          className="h-6 w-6"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M4 6h16M4 12h16M4 18h16"
          />
        </svg>
      </button>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Actions */}
      <div className="flex items-center gap-2">
        <button
          onClick={handleLogout}
          className="btn-secondary text-sm"
        >
          Log out
        </button>
      </div>
    </header>
  );
}
