import UseCases from '@hashicorp/react-use-cases'
import CalloutBlade from 'components/callout-blade'
import CaseStudyCarousel from 'components/case-study-carousel'
import HomepageHero from 'components/homepage-hero'
import StaticDynamicDiagram from 'components/static-dynamic-diagram'

export default function HomePage() {
  return (
    <div className="p-home">
      <HomepageHero
        title="Service Mesh for any runtime or cloud"
        description="Consul automates networking for simple and secure application delivery."
        links={[
          {
            type: 'none',
            text: 'Try HCP Consul',
            url:
              'https://portal.cloud.hashicorp.com/sign-up?utm_source=docs&utm_content=consul_hero',
          },
          {
            type: 'none',
            text: 'Download',
            url: '/downloads',
          },
        ]}
        videos={[
          {
            name: 'UI',
            playbackRate: 2,
            src: [
              {
                srcType: 'mp4',
                url:
                  'https://www.datocms-assets.com/2885/1621637919-consul-ui.mp4',
              },
            ],
          },
          {
            name: 'CLI',
            playbackRate: 2,
            src: [
              {
                srcType: 'mp4',
                url:
                  'https://www.datocms-assets.com/2885/1621637930-consul-cli.mp4',
              },
            ],
          },
        ]}
      />
      <StaticDynamicDiagram
        heading="Service-based networking for dynamic infrastructure"
        diagrams={{
          beforeHeadline: 'Static Infrastructure',
          // @TODO - Convert to a slot w/ JSX markup
          beforeContent:
            '<p class="g-type-body-small">Private datacenters with static IPs, primarily north-south traffic, protected by perimeter security and coarse-grained network segments.</p>\n' +
            '<h4 class="g-type-label"><a class="__permalink-h" href="#traditional-approach" aria-label="traditional approach permalink">»</a><a class="__target-h" id="traditional-approach" aria-hidden></a>Traditional Approach</h4>\n' +
            '<ul>\n' +
            '<li class="g-type-body-small">Static connectivity between services</li>\n' +
            '<li class="g-type-body-small">A fleet of load balancers to route traffic</li>\n' +
            '<li class="g-type-body-small">Ticket driven processes to update network middleware</li>\n' +
            '<li class="g-type-body-small">Firewall rule sprawl to constrict access and insecure flat network zones</li>\n' +
            '</ul>',
          beforeImage: {
            url:
              'https://www.datocms-assets.com/2885/1559693517-static-infrastructure.png',
            alt: 'Static Infrastructure',
          },
          afterHeadline: 'Dynamic Infrastructure',
          // @TODO - Convert to a slot w/ JSX markup
          afterContent:
            '<p class="g-type-body-small">Multiple clouds and private datacenters with dynamic IPs, ephemeral containers, dominated by east-west traffic, no clear network perimeters.</p>\n' +
            '<h4 class="g-type-label"><a class="__permalink-h" href="#consul-approach" aria-label="consul approach permalink">»</a><a class="__target-h" id="consul-approach" aria-hidden></a>Consul Approach</h4>\n' +
            '<ul>\n' +
            '<li class="g-type-body-small">Centralized registry to locate any service</li>\n' +
            '<li class="g-type-body-small">Services discovered and connected with centralized policies</li>\n' +
            '<li class="g-type-body-small">Network automated in service of applications</li>\n' +
            '<li class="g-type-body-small">Zero trust network enforced by identity-based security policies</li>\n' +
            '</ul>',
          afterImage: {
            url:
              'https://www.datocms-assets.com/2885/1559693545-dynamic-infrastructure-4x.png',
            alt: 'Dynamic Infrastructure',
          },
        }}
      />

      <div className="use-cases g-grid-container">
        <h2 className="g-type-display-2">Use Cases</h2>
        <UseCases
          items={[
            {
              title: 'Service Discovery and Health Checking',
              description:
                'Enable services to locate other services running in any environment and provide real-time health status.',
              image: {
                url: require('./img/use-cases/discovery_health_checking.svg?url'),
                format: 'svg',
              },
              link: {
                title: 'Learn more',
                url: '/use-cases/service-discovery-and-health-checking',
              },
            },
            {
              title: 'Network Infrastructure Automation',
              description:
                'Reduce burden of manual, ticket-based networking tasks.',
              image: {
                url: require('./img/use-cases/network_automation.svg?url'),
                format: 'svg',
              },
              link: {
                title: 'Learn more',
                url: '/use-cases/network-infrastructure-automation',
              },
            },
            {
              title: 'Multi-Platform Service Mesh',
              description:
                'Secure, modern application networking across any cloud or runtime.',
              image: {
                url: require('./img/use-cases/service_mesh.svg?url'),
                format: 'svg',
              },
              link: {
                title: 'Learn more',
                url: '/use-cases/multi-platform-service-mesh',
              },
            },
          ]}
        />
      </div>

      <CalloutBlade
        title="Consul Service Mesh"
        callouts={[
          {
            icon: require('./img/kubernetes/logo.svg?include'),
            title: 'For Kubernetes',
            description:
              'Install Consul using Helm charts and deploy using Custom Resource Definitions (CRDs).',
            eyebrow: 'Tutorial',
            link: {
              text: 'Install Consul on your Kubernetes cluster',
              url:
                'https://learn.hashicorp.com/tutorials/consul/service-mesh-deploy?in=consul/gs-consul-service-mesh',
            },
          },
          {
            icon: require('./img/kubernetes/communication-arrows.svg?include'),
            title: 'For Any Runtime',
            description:
              'Secure services and service-to-service communications and connect external services with terminating gateways.',
            eyebrow: 'Tutorial',
            link: {
              text: 'Consul Service Mesh',
              url:
                'https://learn.hashicorp.com/tutorials/consul/service-mesh-deploy-vms?in=consul/developer-mesh',
            },
          },
        ]}
      />

      <CalloutBlade
        title="Consul with HashiCorp Stack"
        callouts={[
          {
            icon: require('./img/stack/consul-and-terraform.svg?include'),
            description:
              'Use the Terraform provider ecosystem to drive relevant changes to your infrastructure based on Consul services.',
            eyebrow: 'Tutorials',
            link: {
              text: 'Consul Terraform Sync',
              url:
                'https://learn.hashicorp.com/tutorials/consul/consul-terraform-sync-intro?in=consul/network-infrastructure-automation',
            },
          },
          {
            icon: require('./img/stack/consul-and-vault.svg?include'),
            description:
              'Integrate Consul with Vault and consul-template to securely store and rotate your encryption key and certificates.',
            eyebrow: 'Tutorials',
            link: {
              text: 'Enforce security with Consul and Vault',
              url:
                'https://learn.hashicorp.com/collections/consul/vault-secure',
            },
          },
          {
            icon: require('./img/stack/consul-and-nomad.svg?include'),
            description:
              'Secure Nomad jobs with Consul Service Mesh and use Traffic Splitting for zero-downtime, blue-green, canary deployments.',
            eyebrow: 'Tutorials',
            link: {
              text: 'Nomad’s integration with Consul',
              url:
                'https://learn.hashicorp.com/collections/nomad/integrate-consul',
            },
          },
        ]}
      />

      <CaseStudyCarousel
        title="Trusted by startups and the world’s largest organizations"
        caseStudies={[
          {
            quote:
              'Consul lets us spread more than 200 microservices over several AKS clusters. Each AKS cluster feeds into a Consul cluster that forms a larger service discovery mesh that allows us to find and connect services in a matter of minutes.',
            caseStudyURL: 'https://www.hashicorp.com/case-studies/mercedes/',
            person: {
              firstName: 'Sriram',
              lastName: 'Govindarajan',
              photo:
                'https://www.datocms-assets.com/2885/1589431834-sriram-govindarajan.jpg',
              title: 'Principal Infrastructure Engineer',
            },
            company: {
              name: 'Mercedes-Benz Research & Development (MBRDNA)',
              logo: require('./img/quotes/mercedes-logo.svg?url'),
            },
          },
          {
            quote:
              'Consul has fully replaced our manual service discovery activities with automated workflows and we’ve repurposed as much as 80% of our Consul staff to other projects because the tool is so reliable, efficient, and intelligent.',
            caseStudyURL:
              'https://www.hashicorp.com/resources/criteo-containers-consul-connect/',
            person: {
              firstName: 'Pierre',
              lastName: 'Souchay',
              photo:
                'https://www.datocms-assets.com/2885/1589431828-pierre-souchay.jpg',
              title: 'Discovery and Security Authorization Lead',
            },
            company: {
              name: 'Criteo',
              logo: require('./img/quotes/criteo-logo.svg?url'),
            },
          },
        ]}
        logoSection={{
          grayBackground: true,
          featuredLogos: [
            {
              companyName: 'Mercedes-Benz Research & Development (MBRDNA)',
              url: require('./img/quotes/mercedes-logo.svg?url'),
            },
            {
              companyName: 'Criteo',
              url: require('./img/quotes/criteo-logo.svg?url'),
            },
            {
              companyName: 'Barclays',
              url: require('./img/quotes/barclays-logo.svg?url'),
            },
            {
              companyName: 'Citadel',
              url: require('./img/quotes/citadel-logo.svg?url'),
            },
            {
              companyName: 'Ample Organics',
              url:
                'https://www.datocms-assets.com/2885/1589354369-ample-organics-logo.png?w=600',
            },
          ],
        }}
      />
    </div>
  )
}
