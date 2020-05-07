import UseCaseLayout from '../../layouts/use-cases'
import TempUseCaseChildren from './_temp-children'

export default function MultiPlatformServiceMeshPage() {
  return (
    <UseCaseLayout
      title="Multi-Platform Service Mesh"
      description="Establish a service mesh between multiple runtime and cloud environments. Create a consistent platform for application networking and security with identity based authorization, L7 traffic management, and service-to-service encryption."
    >
      <TempUseCaseChildren />
    </UseCaseLayout>
  )
}
