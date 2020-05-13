import UseCaseLayout from '../../layouts/use-cases'
import TempUseCaseChildren from './_temp-children'

export default function ServiceDiscoveryAndHealthCheckingPage() {
  return (
    <UseCaseLayout
      title="Service Discovery and Health Checking"
      description="Discover, Register and Resolve services for application workloads across any cloud. Automatically add and remove services based on health checking."
    >
      <TempUseCaseChildren />
    </UseCaseLayout>
  )
}
