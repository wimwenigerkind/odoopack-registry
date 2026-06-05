import type { ReactNode } from "react"
import { Navigate } from "react-router"
import { useMe } from "@/hooks/auth/use-me"

interface RequireAdminProps {
  children: ReactNode
}

export function RequireAdmin({ children }: RequireAdminProps) {
  const { data: user, isLoading } = useMe()

  if (isLoading) return <p>Loading...</p>
  if (!user?.is_admin) return <Navigate to="/" replace />

  return <>{children}</>
}
