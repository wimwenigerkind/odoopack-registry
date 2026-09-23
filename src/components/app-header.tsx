import { LogOut, Search, User, Users } from "lucide-react"
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
  SidebarTrigger,
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
      className="relative w-full max-w-md"
      onSubmit={(e) => {
        e.preventDefault()
        navigate(value ? `/?q=${encodeURIComponent(value)}` : "/")
      }}
    >
      <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
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
          className="shrink-0 rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring"
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

export function AppHeader() {
  const { data: user, isLoading } = useMe()
  return (
    <header className="z-30 flex h-14 shrink-0 items-center gap-3 border-b border-border bg-card px-4">
      <SidebarTrigger className="-ml-1" />
      <div aria-hidden className="mr-1 h-6 w-px shrink-0 bg-border" />
      <SearchBar />
      <div className="ml-auto flex items-center gap-2">
        <ThemeToggle />
        {isLoading ? <Spinner /> : user ? <UserMenu /> : <SignInButton />}
      </div>
    </header>
  )
}
