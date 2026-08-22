import { NavLink } from "react-router"
import { cn } from "@/lib/cn"

const tabs = [
  { to: "/admin/users", label: "Users" },
  { to: "/admin/groups", label: "Groups" },
]

export function AdminNav() {
  return (
    <div className="flex gap-1 border-b border-border">
      {tabs.map((t) => (
        <NavLink
          key={t.to}
          to={t.to}
          className={({ isActive }) =>
            cn(
              "-mb-px border-b-2 px-3 py-2 text-sm font-medium transition-colors",
              isActive
                ? "border-accent text-fg"
                : "border-transparent text-muted hover:text-fg",
            )
          }
        >
          {t.label}
        </NavLink>
      ))}
    </div>
  )
}
