import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { Group } from "@/lib/types"

export function useGroups() {
  const api = useApiClient()
  return useQuery<Group[]>({
    queryKey: queryKeys.groups(),
    queryFn: () => api<Group[]>("/api/v1/groups"),
  })
}

export function useCreateGroup() {
  const api = useApiClient()
  const qc = useQueryClient()
  return useMutation<Group, Error, { name: string }>({
    mutationFn: (req) =>
      api<Group>("/api/v1/groups", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.groups() })
    },
  })
}

export function useDeleteGroup() {
  const api = useApiClient()
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      api<void>(`/api/v1/groups/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.groups() })
    },
  })
}
