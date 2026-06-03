export interface User {
  id: string
  created_at: string
  updated_at: string
  username: string
  identities?: Identity[]
}

export interface Identity {
  id: string
  created_at: string
  updated_at: string
  email: string
  provider: string
  user_id: string
}

export interface ProviderSummary {
  name: string
  type: string
}

export interface ProvidersResponse {
  providers: ProviderSummary[]
}

export type GitProvider = "generic"
export type Visibility = "public" | "private"
export type RefType = "tag" | "branch"
export type VersionStatus = "pending" | "building" | "ready" | "failed"

export interface AddonVersion {
  id: string
  created_at: string
  updated_at: string
  version: string
  ref_type: RefType
  ref_value: string
  status: VersionStatus
  content_hash?: string
  size_bytes?: number
  build_error?: string
  built_at?: string | null
}

export interface Addon {
  id: string
  created_at: string
  updated_at: string
  name: string
  git_provider: GitProvider
  git_url: string
  default_branch: string
  visibility: Visibility
  versions?: AddonVersion[]
}

export interface RegisterAddonRequest {
  name: string
  git_url: string
  git_provider?: GitProvider
  default_branch?: string
  visibility?: Visibility
}

export interface RegisterAddonResponse {
  addon: Addon
  webhook_secret: string
}
