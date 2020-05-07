import UseCases from '@hashicorp/react-use-cases'
import BasicHero from '../../components/basic-hero'
import ConsulEnterpriseComparison from '../../components/consul-enterprise-comparison'
import PrefooterCTA from '../../components/prefooter-cta'
import LearnCallout from '../../components/learn-callout'
import CaseStudyCarousel from '../../components/case-study-carousel'
import ProductFeaturesList from '@hashicorp/react-product-features-list'

export default function HomePage() {
  return (
    <div className="p-home">
      <BasicHero
        brand="consul"
        heading="Service Networking Across Any Cloud or Runtime"
        content="Automate network configurations, discover services, and enable secure connectivity across any cloud or runtime."
        links={[
          {
            text: 'Download',
            url: '/downloads',
            type: 'download',
          },
          {
            text: 'Get Started',
            url: 'https://learn.hashicorp.com/consul',
            type: 'outbound',
          },
        ]}
        backgroundImage
      />

      <ProductFeaturesList
        heading="Why Consul?"
        features={[
          {
            title: 'First Class Kubernetes Experience',
            content:
              'Consul provides a Helm chart for a Kubernetes first experience for Service Discovery and Service Mesh use cases.',
            icon: require('./img/why-consul/kubernetes.svg'),
          },
          {
            title: 'Service Mesh Across Runtime Platforms',
            content:
              'Establish a service mesh between Bare Metal, Virtual Machines, and Kubernetes clusters, across any cloud.',
            icon: require('./img/why-consul/service-mesh-runtime.svg'),
          },
          {
            title: 'Dynamic Load Balancing Configurations',
            content:
              'Use Consul to automate service updates to popular load balancers (F5, NGINX, HAProxy) and eliminate manual configuration.',
            icon: require('./img/why-consul/dynamic-load-balancing.svg'),
          },
          {
            title: 'Secure, Multi-Cloud Service Networking',
            content:
              'Secure any service running in any environment. Consul enables users to automate and secure service to service communication.',
            icon: require('./img/why-consul/cloud.svg'),
          },
          {
            title: 'Service Discovery with Health Checking',
            content:
              'Consul enables detecting the deployment of new services, changes to existing ones, and provides real time agent health to reduce downtime.',
            icon: require('./img/why-consul/health.svg'),
          },
          {
            title: 'Robust Ecosystem',
            content:
              'Consul offers support for and integrations with many popular DevOps and Networking tools.',
            icon: require('./img/why-consul/world.svg'),
          },
        ]}
      />

      <CaseStudyCarousel
        title="Trusted by startups and the world’s largest organizations"
        caseStudies={Array(5).fill({
          quote:
            "Here's a quote about Consul, Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore.",
          caseStudyURL: 'https://www.hashicorp.com',
          person: {
            firstName: 'Brandon',
            lastName: 'Romano',
            photo:
              'https://avatars1.githubusercontent.com/u/2105067?s=460&u=d20ade7241340fb1a277b55816f0a5336a26d95c&v=4',
            title: 'A Real Person',
          },
          company: {
            name: 'HashiCorp',
            logo:
              'https://www.datocms-assets.com/2885/1503088697-blog-hashicorp.svg',
          },
        })}
        logoSection={{
          grayBackground: true,
          featuredLogos: Array(6).fill({
            companyName: 'HashiCorp',
            url:
              'https://www.datocms-assets.com/2885/1503088697-blog-hashicorp.svg',
          }),
        }}
      />

      <div className="use-cases g-grid-container">
        <h2 className="g-type-display-2">Use Cases</h2>
        <UseCases
          items={[
            {
              title: 'Network Middleware Automation',
              description:
                'Reduce burden of manual, ticket-based networking tasks.',
              image: {
                url: require('./img/use-cases/network-middleware-automation.png?url'),
                format: 'png',
              },
              link: {
                title: 'Learn more',
                url: '/use-cases/network-middleware-automation',
              },
            },
            {
              title: 'Multi-Platform Service Mesh',
              description:
                'Secure, modern application networking across any cloud or runtime.',
              image: {
                url: require('./img/use-cases/multi-platform-service-mesh.png?url'),
                format: 'png',
              },
              link: {
                title: 'Learn more',
                url: '/use-cases/multi-platform-service-mesh',
              },
            },
            {
              title: 'Service Discovery & Health Checks',
              description:
                'Enable services to locate other services running in any environment and provide real-time health status.',
              image: {
                url: require('./img/use-cases/service-discovery-and-health-checks.png?url'),
                format: 'png',
              },
              link: {
                title: 'Learn more',
                url: '/use-cases/service-discovery-and-health-checks',
              },
            },
          ]}
        />
      </div>

      <LearnCallout
        headline="Learn the latest Consul skills"
        brand="consul"
        items={[
          {
            title: 'Getting Started',
            category: 'Step-by-Step Guides',
            time: '48 mins',
            link:
              'https://learn.hashicorp.com/consul?track=getting-started#getting-started',
            image: require('./img/learn/getting-started.svg?url'),
          },
          {
            title: 'Run Consul on Kubernetes',
            category: 'Step-by-Step Guides',
            time: '142 mins',
            link:
              'https://learn.hashicorp.com/consul?track=kubernetes#kubernetes',
            image: require('./img/learn/getting-started.svg?url'),
          },
        ]}
      />
      <ConsulEnterpriseComparison />
      <PrefooterCTA />
    </div>
  )
}
