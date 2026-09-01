export interface User {
  id: string
  created_at: string
  updated_at: string
  username: string
  email?: string
  gravatar_hash?: string
  is_admin: boolean
  identities?: Identity[]
}

export interface Group {
  id: string
  created_at: string
  updated_at: string
  name: string
}

export interface GroupMembership {
  group_id: string
  user_id: string
  created_at: string
}

export interface GroupAddonAccess {
  group_id: string
  addon_id: string
  created_at: string
}

export interface Identity {
  id: string
  created_at: string
  updated_at: string
  provider: string
  user_id: string
}

export interface ApiToken {
  id: string
  created_at: string
  updated_at: string
  user_id: string
  name: string
  token: string
  last_used_at?: string | null
}

export interface CreateTokenRequest {
  name: string
}

export interface OAuthIntegration {
  id: string
  created_at: string
  updated_at: string
  provider: string
  owner_id: string
  account_name: string
  expires_at?: string | null
  scope?: string
  hook_secret?: string
}

export interface IntegrationProviderSummary {
  name: string
  type: string
}

export interface IntegrationProvidersResponse {
  providers: IntegrationProviderSummary[]
}

export interface ProviderSummary {
  name: string
  type: string
}

export interface ProvidersResponse {
  providers: ProviderSummary[]
}

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
  depends?: string[]
  depends_resolved?: ResolvedDep[]
  manifest_version?: string
  series?: string
  is_latest: boolean
}

export interface ResolvedDep {
  module: string
  addon_id?: string
  name?: string
  access: "ok" | "external" | "forbidden"
}

export interface ReadmeResponse {
  html: string
  updated_at: string
}

export interface Repo {
  id: string
  created_at: string
  updated_at: string
  git_url: string
  default_branch: string
  owner_id: string
  owner?: User
  integration_id?: string | null
  integration?: OAuthIntegration | null
  addons?: Addon[]
}

export interface Addon {
  id: string
  created_at: string
  updated_at: string
  name: string
  repo_id: string
  repo?: Repo
  subpath?: string
  visibility: Visibility
  versions?: AddonVersion[]
}

export interface RegisterAddonRequest {
  name: string
  git_url: string
  default_branch?: string
  subpath?: string
  visibility?: Visibility
  integration_id?: string
}

export interface UpdateAddonRequest {
  name?: string
  subpath?: string
  visibility?: Visibility
}

export interface UpdateRepoRequest {
  default_branch?: string
  integration_id?: string | null
}

