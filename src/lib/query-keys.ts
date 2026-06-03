export const queryKeys = {
  me: () => ["me"] as const,
  providers: () => ["auth", "providers"] as const,
  addons: () => ["addons"] as const,
  addon: (id: string) => ["addons", id] as const,
}