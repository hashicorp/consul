import UseCaseLayout from '../../layouts/use-cases'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'
import FeaturedSlider from '@hashicorp/react-featured-slider'

export default function NetworkMiddlewareAutomationPage() {
  return (
    <UseCaseLayout
      title="Network Middleware Automation"
      description="Reduce time to deploy and eliminate manual processes by automating complex networking tasks. Developers can rollout new services, scale up and down, and gracefully handle failure without operator intervention."
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
          url: require('./img/ecosystem.svg?url'),
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

      <FeaturedSlider
        heading="Case Study"
        theme="dark"
        brand="consul"
        features={[
          {
            logo: {
              url: require('./img/mercedes-logo.svg?url'),
              alt: 'Mercedes-Benz',
            },
            image: {
              url: require('./img/mercedes-card.jpg?url'),
              alt: 'Mercedes-Benz Case Study',
            },
            heading: 'On the Road Again',
            content:
              'How Mercedes-Benz delivers on service networking to accelerate delivery of its next-gen connected vehicles.',
            link: {
              text: 'Read Case Study',
              url: 'https://www.hashicorp.com/case-studies/mercedes/',
              type: 'outbound',
            },
          },
        ]}
      />
    </UseCaseLayout>
  )
}
