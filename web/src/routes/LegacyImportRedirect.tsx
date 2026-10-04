import { Navigate, useLocation, useParams } from 'react-router'

export function LegacyImportRedirect() {
  const { sessionId } = useParams()
  const location = useLocation()
  return (
    <Navigate
      replace
      to={`/ai-wizard${sessionId ? `/${sessionId}` : ''}${location.search}`}
    />
  )
}
