import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type {
  RegisterAddonRequest,
  RegisterAddonResponse,
} from "@/lib/types"

export function useRegisterAddon() {
  const api = useApiClient()
  const qc = useQueryClient()

  return useMutation<RegisterAddonResponse, Error, RegisterAddonRequest>({
    mutationFn: (req) =>
      api<RegisterAddonResponse>("/api/v1/addons", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.addons() })
    },
  })
}
