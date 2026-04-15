import { lazy, Suspense } from 'react'
import { BrowserRouter, Route, Routes } from 'react-router-dom'

import Layout from '@/components/layout'
import { TooltipProvider } from '@/components/ui/tooltip'
import HomePage from '@/pages/home-page'

// Lazy-load non-home routes to keep the initial bundle small.
// ProjectPage is the heaviest (xterm, diff viewer, recharts).
const ProjectPage = lazy(() => import('@/pages/project-page'))
const AccessPage = lazy(() => import('@/pages/access-page'))
const AuditPage = lazy(() => import('@/pages/audit-page'))

/** Root application component with client-side routing. */
export default function App() {
  return (
    <TooltipProvider>
      <BrowserRouter>
        <Suspense>
          <Routes>
            <Route element={<Layout />}>
              <Route path="/" element={<HomePage />} />
              <Route path="/projects/:id/:agentType" element={<ProjectPage />} />
              <Route path="/access" element={<AccessPage />} />
              <Route path="/audit" element={<AuditPage />} />
            </Route>
          </Routes>
        </Suspense>
      </BrowserRouter>
    </TooltipProvider>
  )
}
