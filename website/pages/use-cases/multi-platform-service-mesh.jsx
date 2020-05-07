import UseCaseLayout from '../../layouts/use-cases'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'

export default function MultiPlatformServiceMeshPage() {
  return (
    <UseCaseLayout
      title="Multi-Platform Service Mesh"
      description="Establish a service mesh between multiple runtime and cloud environments. Create a consistent platform for application networking and security with identity based authorization, L7 traffic management, and service-to-service encryption."
    >
      <div className="with-border">
        <TextSplitWithImage
          textSplit={{
            heading: 'Multi-region, multi-cloud',
            content:
              'Consul’s flexible architecture allows it to be deployed in any environment, in any region, on any cloud.',
            textSide: 'left',
            links: [
              {
                text: 'Learn More',
                url:
                  'https://learn.hashicorp.com/consul?track=datacenter-deploy#datacenter-deploy',
                type: 'outbound',
              },
            ],
          }}
          image={{
            url:
              'https://www.datocms-assets.com/2885/1588822376-multi-region.png',
            alt: '',
          }}
        />
      </div>

      <div className="with-border">
        <TextSplitWithImage
          textSplit={{
            heading: 'Multi-region, multi-cloud',
            content:
              'Consul’s flexible architecture allows it to be deployed in any environment, in any region, on any cloud.',
            textSide: 'right',
            links: [
              {
                text: 'Learn More',
                url:
                  'https://learn.hashicorp.com/consul?track=datacenter-deploy#datacenter-deploy',
                type: 'outbound',
              },
            ],
          }}
          image={{
            url:
              'https://www.datocms-assets.com/2885/1588822376-multi-region.png',
            alt: '',
          }}
        />
      </div>
    </UseCaseLayout>
  )
}
