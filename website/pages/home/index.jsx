import UseCases from '@hashicorp/react-use-cases'
import ProductFeaturesList from '@hashicorp/react-product-features-list'
import MiniCTA from 'components/mini-cta'
import HcpCalloutSection from 'components/hcp-callout-section'
import CtaHero from 'components/cta-hero'
import CalloutBlade from 'components/callout-blade'
import ConsulEnterpriseComparison from 'components/enterprise-comparison/consul'
import PrefooterCTA from 'components/prefooter-cta'
import CaseStudyCarousel from 'components/case-study-carousel'

export default function HomePage() {
  return (
    <div className="p-home">
      <CtaHero
        title="Service Mesh for any runtime or cloud"
        description="Consul automates networking for simple and secure application delivery."
        links={[
          {
            type: 'none',
            text: 'Download Consul',
            url: '/downloads',
          },
          {
            type: 'none',
            text: 'Explore Tutorials',
            url: 'https://learn.hashicorp.com/consul',
          },
        ]}
        cta={{
          title: 'Try HCP Consul',
          description:
            'A fully managed service mesh to discover and securely connect any service.',
          link: {
            text: 'Sign Up',
            url:
              'https://portal.cloud.hashicorp.com/sign-up?utm_source=consul_io&utm_content=hero',
          },
        }}
      />

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

      <ProductFeaturesList
        heading="Why Consul?"
        features={[
          {
            title: 'Secure, Multi-Cloud Service Networking',
            content:
              'Secure services running in any environment leveraging intention based policies and automatic mTLS encryption between service mesh resources',
            icon: require('./img/why-consul/consul_features_cloud.svg'),
            link: {
              type: 'inbound',
              text: 'Learn more',
              url:
                'https://learn.hashicorp.com/tutorials/consul/kubernetes-secure-agents',
            },
          },
          {
            title: 'Dynamic Load Balancing',
            content:
              'Resolve discovered services through integrated DNS. Automate 3rd party load balancers (F5, NGINX, HAProxy). Eliminate manual configuration of network devices.',
            icon: require('./img/why-consul/consul_features_gear.svg'),
            link: {
              type: 'inbound',
              text: 'Learn more',
              url:
                'https://learn.hashicorp.com/collections/consul/load-balancing',
            },
          },
          {
            title: 'Service Discovery with Health Checking',
            content:
              'Consul enables detecting the deployment of new services, changes to existing ones, and provides real time agent health to reduce downtime.',
            icon: require('./img/why-consul/consul_features_health.svg'),
            link: {
              type: 'inbound',
              text: 'Learn more',
              url:
                'https://learn.hashicorp.com/tutorials/consul/service-registration-health-checks',
            },
          },
          {
            title: 'Robust Ecosystem',
            content:
              'Consul offers support for and integrations with many popular DevOps and Networking tools.',
            icon: require('./img/why-consul/consul_features_world.svg'),
            link: {
              type: 'inbound',
              text: 'Learn more',
              url: '/docs/integrate/partnerships',
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
      <MiniCTA
        title="Are you using Consul in production?"
        link={{
          text: 'Share your success story and receive special Consul swag.',
          url:
            'https://docs.google.com/forms/d/1B-4XlRndv2hX9G4Gt2dMnJBqilctrrof7dfpyQ1EVIg/edit',
          type: 'outbound',
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

      <HcpCalloutSection
        id="cloud-offerings"
        title="HCP Consul"
        chin="Available on AWS"
        description="A fully managed service mesh to discover and securely connect any service."
        image={require('./img/hcp_consul.svg?url')}
        links={[
          {
            text: 'Learn More',
            url:
              'https://cloud.hashicorp.com/?utm_source=consul_io&utm_content=hcp_consul_detail',
          },
          {
            text: 'Looking for Consul Service on Azure?',
            url: 'https://www.hashicorp.com/products/consul/service-on-azure',
            type: 'inbound',
          },
        ]}
      />

      <ConsulEnterpriseComparison />
      <PrefooterCTA />
    </div>
  )
}
