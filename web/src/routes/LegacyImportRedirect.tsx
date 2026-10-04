import { Navigate, useLocation, useParams } from 'react-router'

export function LegacyImportRedirect() {
  const { sessionId, importView } = useParams()
  const location = useLocation()
  return (
    <Navigate
      replace
      to={`/ai-wizard${sessionId ? `/${sessionId}` : ''}${importView ? `/${importView}` : ''}${location.search}`}
    />
  )
}
