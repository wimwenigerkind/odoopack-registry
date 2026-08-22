import { LogOut, Package, Search, User, Users } from "lucide-react"
import { useState } from "react"
import { Link, useLocation, useNavigate, useSearchParams } from "react-router"
import { Avatar } from "@/components/avatar"
import { ThemeToggle } from "@/components/theme-toggle"
import {
  buttonVariants,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Input,
  Spinner,
} from "@/components/ui"
import { useMe } from "@/hooks/auth/use-me"
import { useLogout } from "@/hooks/auth/use-logout"

function SearchBar() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const [value, setValue] = useState(params.get("q") ?? "")
  return (
    <form
      className="relative min-w-0 flex-1"
      onSubmit={(e) => {
        e.preventDefault()
        navigate(value ? `/?q=${encodeURIComponent(value)}` : "/")
      }}
    >
      <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted" />
      <Input
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Search addons…"
        className="h-9 pl-9"
      />
    </form>
  )
}

function UserMenu() {
  const { data: user } = useMe()
  const logout = useLogout()
  if (!user) return null
  const displayName = user.username || user.email || "Account"
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          className="shrink-0 rounded-full outline-none focus-visible:ring-2 focus-visible:ring-accent"
          aria-label="Account menu"
        >
          <Avatar hash={user.gravatar_hash} size={32} />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuLabel>{displayName}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/profile">
            <User className="size-4" /> Profile
          </Link>
        </DropdownMenuItem>
        {user.is_admin && (
          <>
            <DropdownMenuItem asChild>
              <Link to="/admin/users">
                <Users className="size-4" /> Users
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <Link to="/admin/groups">
                <Users className="size-4" /> Groups
              </Link>
            </DropdownMenuItem>
          </>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => logout.mutate()}>
          <LogOut className="size-4" /> Logout
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function SignInButton() {
  const location = useLocation()
  const returnTo = location.pathname + location.search
  return (
    <Link
      to={`/login?return_to=${encodeURIComponent(returnTo)}`}
      className={buttonVariants({ size: "sm" })}
    >
      Sign in
    </Link>
  )
}

export function TopBar() {
  const { data: user, isLoading } = useMe()
  return (
    <header className="sticky top-0 z-40 border-b border-border bg-surface/80 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-[1100px] items-center gap-3 px-4">
        <Link
          to="/"
          className="flex shrink-0 items-center gap-2 font-semibold"
        >
          <Package className="size-5 text-accent" />
          <span className="hidden sm:inline">Odoopack</span>
        </Link>
        <SearchBar />
        <ThemeToggle />
        {isLoading ? <Spinner /> : user ? <UserMenu /> : <SignInButton />}
      </div>
    </header>
  )
}
