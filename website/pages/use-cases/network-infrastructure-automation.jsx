import UseCaseLayout from 'components/use-cases-layout'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'

export default function NetworkInfrastructureAutomationPage() {
  return (
    <UseCaseLayout
      title="Network Infrastructure Automation"
      description="Reduce the time to deploy applications and eliminate manual processes by automating complex networking tasks. Enable operators to easily deploy, manage and optimize network infrastructure."
      guideLink="https://learn.hashicorp.com/consul?track=integrations"
    >
      <TextSplitWithImage
        textSplit={{
          heading: 'Dynamic Load Balancing',
          content:
            'Consul can automatically provide service updates to many popular load balancers eliminating the need for manual updates.',
          textSide: 'right',
          links: [
            {
              text: 'Learn More',
              url:
                'https://learn.hashicorp.com/collections/consul/integrations',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/network-automation/load-balancing.png'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Automated Firewalling',
          content:
            'Using Consul-Terraform-Sync to automate security updates, organizations can elevate their security posture and adopt fine-grained access policies.',
          textSide: 'left',
          links: [
            {
              text: 'Learn More',
              url: '/docs/nia',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/network-automation/firewalling.png'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Health Checks Visibility',
          content:
            'Consul enables operators to gain real-time insights into the service definitions, health, and location of applications supported by the network.',
          textSide: 'right',
          links: [
            {
              text: 'Learn More',
              url:
                'https://www.hashicorp.com/integrations?product=consul&type=sdn',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/network-automation/health-checks-visibility.png'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Extend through Ecosystem',
          content:
            'Consul’s open API enables integrations with many popular networking technologies.',
          textSide: 'left',
          links: [
            {
              text: 'Read More',
              url: '/docs/integrate/nia-integration',
              type: 'inbound',
            },
          ],
        }}
        image={{
          url: require('./img/network-automation/ecosystem-extend.png'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Flexible Architecture',
          content:
            'Consul can be deployed in any environment, across any cloud or runtime.',
          textSide: 'right',
          links: [
            {
              text: 'Learn More',
              url:
                'https://learn.hashicorp.com/consul/datacenter-deploy/reference-architecture',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/network-automation/architecture-extend.png'),
        }}
      />

      <div className="with-border">
        <TextSplitWithImage
          textSplit={{
            heading: 'Reduce Downtime and Outages',
            content:
              'Use Consul to automate networking tasks, reducing risk of outages from manual errors and driving down ticket driven operations.',
            textSide: 'left',
            links: [
              {
                text: 'Learn More',
                url:
                  'https://learn.hashicorp.com/tutorials/consul/recovery-outage?in=consul/datacenter-operations',
                type: 'outbound',
              },
            ],
          }}
          image={{
            url: require('./img/discovery-health-checking/consul_screenshot@2x.png?url'),
          }}
        />
      </div>
    </UseCaseLayout>
  )
}
