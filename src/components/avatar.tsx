interface AvatarProps {
  hash: string | undefined | null
  size?: number
  alt?: string
}

export function Avatar({ hash, size = 32, alt }: AvatarProps) {
  if (!hash) return null
  return (
    <img
      src={`https://www.gravatar.com/avatar/${hash}?s=${size * 2}&d=identicon`}
      width={size}
      height={size}
      alt={alt ?? "avatar"}
    />
  )
}
