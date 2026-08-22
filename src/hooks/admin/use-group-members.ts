import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { GroupMembership } from "@/lib/types"

export function useGroupMembers(groupID: string) {
  const api = useApiClient()
  return useQuery<GroupMembership[]>({
    queryKey: queryKeys.groupMembers(groupID),
    queryFn: () =>
      api<GroupMembership[]>(`/api/v1/groups/${groupID}/members`),
    enabled: !!groupID,
  })
}

export function useAddGroupMember(groupID: string) {
  const api = useApiClient()
  const qc = useQueryClient()
  return useMutation<void, Error, { user_id: string }>({
    meta: { successMessage: "Member added" },
    mutationFn: (req) =>
      api<void>(`/api/v1/groups/${groupID}/members`, {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.groupMembers(groupID) })
    },
  })
}

export function useRemoveGroupMember(groupID: string) {
  const api = useApiClient()
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    meta: { successMessage: "Member removed" },
    mutationFn: (userID) =>
      api<void>(`/api/v1/groups/${groupID}/members/${userID}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.groupMembers(groupID) })
    },
  })
}
