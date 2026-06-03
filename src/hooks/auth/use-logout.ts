import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { useApiClient } from "@/lib/api"

export function useLogout() {
  const api = useApiClient()
  const qc = useQueryClient()
  const navigate = useNavigate()

  return useMutation({
    mutationFn: () => api<void>("/auth/logout", { method: "POST" }),
    onSuccess: () => {
      qc.removeQueries()
      navigate("/")
    },
  })
}
