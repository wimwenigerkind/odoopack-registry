import { useQuery } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"

export function useSeries() {
  const api = useApiClient()
  return useQuery({
    queryKey: queryKeys.series(),
    queryFn: () => api<{ series: string[] }>("/api/v1/series"),
    select: (data) => data.series,
  })
}
