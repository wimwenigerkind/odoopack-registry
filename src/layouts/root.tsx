import { Outlet } from "react-router"
import { AppHeader } from "@/components/app-header"
import { AppSidebar } from "@/components/app-sidebar"
import { SidebarInset, SidebarProvider, TooltipProvider } from "@/components/ui"

export default function RootLayout() {
  return (
    <TooltipProvider delayDuration={0}>
      <SidebarProvider>
        <AppSidebar />
        <SidebarInset>
          <AppHeader />
          <main className="flex-1 px-4 py-8 sm:px-6 lg:px-8">
            <div className="mx-auto w-full max-w-[1100px]">
              <Outlet />
            </div>
          </main>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  )
}
