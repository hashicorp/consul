import UseCaseLayout from '../../layouts/use-cases'
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
                'https://learn.hashicorp.com/consul?track=integrations#integrations',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/dynamic-load-balancing.svg?url'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Extend through Ecosystem',
          content:
            'Consul’s open API enables integrations with many popular networking tools.',
          textSide: 'left',
          links: [
            {
              text: 'Read More',
              url: 'https://www.consul.io/docs/partnerships/index.html',
              type: 'inbound',
            },
          ],
        }}
        image={{
          url: require('./img/extend-through-ecosystem.svg?url'),
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
          url: require('./img/flexible-architecture.svg?url'),
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
                  'https://learn.hashicorp.com/consul?track=integrations#integrations',
                type: 'outbound',
              },
            ],
          }}
          image={{
            url: require('./img/services-screenshot.png?url'),
          }}
        />
      </div>
    </UseCaseLayout>
  )
}
