import {StrictMode} from 'react'
import {createRoot} from 'react-dom/client'
import '@fontsource-variable/inter'
import './index.css'
import App from './App.tsx'
import {QueryClientProvider} from '@tanstack/react-query'
import {queryClient} from '@/lib/query-client'
import {AppToaster} from '@/components/app-toaster'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App/>
      <AppToaster/>
    </QueryClientProvider>
  </StrictMode>,
)
