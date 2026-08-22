import { Moon, Sun } from "lucide-react"
import { useState } from "react"
import { Button } from "@/components/ui"
import { setTheme } from "@/lib/theme"

export function ThemeToggle() {
  const [isDark, setIsDark] = useState(() =>
    document.documentElement.classList.contains("dark"),
  )
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label="Toggle theme"
      onClick={() => {
        const next = !isDark
        setTheme(next ? "dark" : "light")
        setIsDark(next)
      }}
    >
      {isDark ? <Sun className="size-5" /> : <Moon className="size-5" />}
    </Button>
  )
}
