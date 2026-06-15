import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"

export function useDeleteVersion(addonID: string) {
  const api = useApiClient()
  const qc = useQueryClient()

  return useMutation<void, Error, string>({
    mutationFn: (version) =>
      api<void>(
        `/api/v1/addons/${addonID}/versions/${encodeURIComponent(version)}`,
        { method: "DELETE" },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.addon(addonID) })
    },
  })
}
