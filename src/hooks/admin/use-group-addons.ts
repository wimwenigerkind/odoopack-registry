import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { GroupAddonAccess } from "@/lib/types"

export function useGroupAddons(groupID: string) {
  const api = useApiClient()
  return useQuery<GroupAddonAccess[]>({
    queryKey: queryKeys.groupAddons(groupID),
    queryFn: () =>
      api<GroupAddonAccess[]>(`/api/v1/groups/${groupID}/addons`),
    enabled: !!groupID,
  })
}

export function useGrantGroupAddon(groupID: string) {
  const api = useApiClient()
  const qc = useQueryClient()
  return useMutation<void, Error, { addon_id: string }>({
    meta: { successMessage: "Access granted" },
    mutationFn: (req) =>
      api<void>(`/api/v1/groups/${groupID}/addons`, {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.groupAddons(groupID) })
    },
  })
}

export function useRevokeGroupAddon(groupID: string) {
  const api = useApiClient()
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    meta: { successMessage: "Access revoked" },
    mutationFn: (addonID) =>
      api<void>(`/api/v1/groups/${groupID}/addons/${addonID}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.groupAddons(groupID) })
    },
  })
}
