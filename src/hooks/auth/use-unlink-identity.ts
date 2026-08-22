import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"

export function useUnlinkIdentity() {
  const api = useApiClient()
  const qc = useQueryClient()

  return useMutation<void, Error, string>({
    meta: { successMessage: "Account unlinked" },
    mutationFn: (id) =>
      api<void>(`/api/v1/me/identities/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.me() })
    },
  })
}
