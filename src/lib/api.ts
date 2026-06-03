import {ApiError} from "@/lib/api-error"

export const BACKEND_BASE = import.meta.env.VITE_BACKEND_BASE_URL ?? ""

export function useApiClient() {
  return async <T, >(path: string, options?: RequestInit): Promise<T> => {
    const res = await fetch(`${BACKEND_BASE}${path}`, {
      credentials: "include",
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...options?.headers,
      },
    })

    if (!res.ok) {
      const body = (await res.json().catch(() => null)) as
        | { error?: string }
        | null
      throw new ApiError(res.status, body?.error ?? res.statusText)
    }

    if (res.status === 204) return undefined as T
    return res.json() as Promise<T>
  }
}
