import { Outlet } from "react-router"
import { AppHeader } from "@/components/app-header"
import { AppSidebar } from "@/components/app-sidebar"
import { SidebarInset, SidebarProvider, TooltipProvider } from "@/components/ui"

export default function RootLayout() {
  return (
    <TooltipProvider delayDuration={0}>
      <SidebarProvider className="h-svh overflow-hidden">
        <AppSidebar />
        <SidebarInset className="min-h-0 overflow-hidden">
          <AppHeader />
          <div className="min-h-0 flex-1 overflow-y-auto">
            <div className="w-full px-4 py-8 sm:px-6 lg:px-8">
              <Outlet />
            </div>
          </div>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  )
}
