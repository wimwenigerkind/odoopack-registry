import {useQuery} from "@tanstack/react-query"
import {useApiClient} from "@/lib/api"
import {queryKeys} from "@/lib/query-keys"
import type {Addon} from "@/lib/types.ts";

export function useAddons() {
  const api = useApiClient()
  return useQuery<Addon[]>({
    queryKey: queryKeys.addons(),
    queryFn: () => api<Addon[]>("/api/v1/addons"),
  })
}