export const queryKeys = {
  me: () => ["me"] as const,
  providers: () => ["auth", "providers"] as const,
  addons: () => ["addons"] as const,
  addon: (id: string) => ["addons", id] as const,

  users: () => ["admin", "users"] as const,
  groups: () => ["admin", "groups"] as const,
  group: (id: string) => ["admin", "groups", id] as const,
  groupMembers: (id: string) => ["admin", "groups", id, "members"] as const,
  groupAddons: (id: string) => ["admin", "groups", id, "addons"] as const,

  tokens: () => ["me", "tokens"] as const,
  integrations: () => ["me", "integrations"] as const,
  integrationProviders: () => ["integrations", "providers"] as const,
}
