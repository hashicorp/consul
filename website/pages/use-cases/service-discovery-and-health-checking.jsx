import UseCaseLayout from '../../layouts/use-cases'
import TempUseCaseChildren from './_temp-children'

export default function ServiceDiscoveryAndHealthCheckingPage() {
  return (
    <UseCaseLayout
      title="Service Discovery and Health Checking"
      description="Service registry, integrated health checks, and DNS and HTTP interfaces enable any service to discover and be discovered by other services"
    >
      <TempUseCaseChildren />
    </UseCaseLayout>
  )
}
