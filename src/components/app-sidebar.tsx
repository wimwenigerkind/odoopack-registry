import {
  Package,
  Plus,
  User,
  Users,
  UsersRound,
  LibraryBig,
} from "lucide-react"
import type { ComponentType } from "react"
import { Link, useLocation } from "react-router"
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui"
import { useMe } from "@/hooks/auth/use-me"

type NavItem = {
  title: string
  to: string
  icon: ComponentType<{ className?: string }>
  match?: (pathname: string) => boolean
}

type NavGroup = {
  label: string
  items: NavItem[]
}

const startsWith = (prefix: string) => (pathname: string) =>
  pathname === prefix || pathname.startsWith(prefix + "/")

export function AppSidebar() {
  const { pathname } = useLocation()
  const { data: user } = useMe()

  const groups: NavGroup[] = [
    {
      label: "Discover",
      items: [
        {
          title: "Addons",
          to: "/",
          icon: LibraryBig,
          match: (p) => p === "/" || p.startsWith("/addons"),
        },
      ],
    },
  ]

  if (user) {
    groups.push({
      label: "Manage",
      items: [
        { title: "Register addon", to: "/addons/new", icon: Plus },
        { title: "Profile", to: "/profile", icon: User },
      ],
    })
  }

  if (user?.is_admin) {
    groups.push({
      label: "Administration",
      items: [
        {
          title: "Users",
          to: "/admin/users",
          icon: Users,
          match: startsWith("/admin/users"),
        },
        {
          title: "Groups",
          to: "/admin/groups",
          icon: UsersRound,
          match: startsWith("/admin/groups"),
        },
      ],
    })
  }

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <Link
          to="/"
          className="flex items-center gap-2 px-2 py-1.5 font-semibold"
        >
          <Package className="size-5 shrink-0 text-primary" />
          <span className="truncate group-data-[collapsible=icon]:hidden">
            Odoopack
          </span>
        </Link>
      </SidebarHeader>
      <SidebarContent>
        {groups.map((group) => (
          <SidebarGroup key={group.label}>
            <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {group.items.map((item) => {
                  const active = item.match
                    ? item.match(pathname)
                    : pathname === item.to
                  return (
                    <SidebarMenuItem key={item.to}>
                      <SidebarMenuButton
                        asChild
                        isActive={active}
                        tooltip={item.title}
                      >
                        <Link to={item.to}>
                          <item.icon className="size-4" />
                          <span>{item.title}</span>
                        </Link>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  )
                })}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ))}
      </SidebarContent>
      <SidebarRail />
    </Sidebar>
  )
}
