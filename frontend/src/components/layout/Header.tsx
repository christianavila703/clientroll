interface HeaderProps {
  onMenuToggle: () => void;
}

export default function Header({ onMenuToggle }: HeaderProps) {
  return (
    <header className="sticky top-0 z-30 flex h-[60px] items-center gap-3 border-b border-gray-200 bg-white px-4 lg:px-6">
      {/* Mobile hamburger */}
      <button
        onClick={onMenuToggle}
        className="flex h-9 w-9 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 lg:hidden"
        aria-label="Toggle menu"
      >
        <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 6h16M4 12h16M4 18h16" />
        </svg>
      </button>

      {/* Search */}
      <div className="hidden sm:flex flex-1 max-w-md">
        <div className="relative w-full">
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            type="text"
            placeholder="Search..."
            className="w-full rounded-lg border border-gray-200 bg-gray-50 pl-9 pr-3 py-2 text-sm text-gray-700 placeholder:text-gray-400 focus:border-primary-300 focus:bg-white focus:outline-none focus:ring-1 focus:ring-primary-300 transition-colors"
          />
        </div>
      </div>

      <div className="flex-1 sm:hidden" />

      {/* Right actions */}
      <div className="flex items-center gap-1.5">
        {/* Notifications bell */}
        <button className="relative flex h-9 w-9 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 transition-colors">
          <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
          </svg>
        </button>
      </div>
    </header>
  );
}
