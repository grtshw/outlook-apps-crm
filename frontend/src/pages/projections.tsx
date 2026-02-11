import { ProjectionsPage } from '@/components/ui/projections-page'
import {
  projectAll,
  getProjectionLogs,
  getProjectionConsumers,
  toggleProjectionConsumer,
  getProjectionProgress,
} from '@/lib/api'

export default function CRMProjectionsPage() {
  return (
    <ProjectionsPage
      mode="provider"
      entityNoun="contacts + organisations"
      projectAll={projectAll}
      getProjectionLogs={getProjectionLogs}
      getProjectionConsumers={getProjectionConsumers}
      toggleProjectionConsumer={toggleProjectionConsumer}
      getProjectionProgress={getProjectionProgress}
    />
  )
}
