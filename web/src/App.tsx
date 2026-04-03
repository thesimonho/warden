import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { TooltipProvider } from '@/components/ui/tooltip'
import Layout from '@/components/layout'
import HomePage from '@/pages/home-page'
import ProjectPage from '@/pages/project-page'
import AccessPage from '@/pages/access-page'
import AuditPage from '@/pages/audit-page'

/** Root application component with client-side routing. */
export default function App() {
  return (
    <TooltipProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<HomePage />} />
            <Route path="/projects/:id/:agentType" element={<ProjectPage />} />
            <Route path="/access" element={<AccessPage />} />
            <Route path="/audit" element={<AuditPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </TooltipProvider>
  )
}
