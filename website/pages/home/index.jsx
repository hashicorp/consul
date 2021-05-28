import LearnCallout from '@hashicorp/react-learn-callout'
import SteppedFeatureList from '@hashicorp/react-stepped-feature-list'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'
import CodeBlock from '@hashicorp/react-code-block'
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
        <h2 className="g-type-display-2">Why Consul?</h2>
        <UseCases
          items={[
            {
              title: 'Microservice Based Networking',
              description:
                'Simplify developer interaction with applications using Consul service mesh and a API driven approach: from ingress, to blue-green and canary deployments, while including fine-grained observability and health checks for microservice architectures.',
              image: {
                url: require('./img/use-cases/service_mesh.svg?url'),
                format: 'svg',
              },
              link: {
                title: 'Learn more about service mesh with Consul',
                url: '/use-cases/multi-platform-service-mesh',
              },
            },
            {
              title: 'Secure Service-to-Service Access',
              description:
                'Enable secure services access and communication across any network with identity-driven, time-based controls.. Enforce patterns and policies in VM and container-based environments and across public and private clouds. Move towards a Zero Trust posture for network security.',
              image: {
                url: '',
                format: 'svg',
              },
              link: {
                title: 'Learn more about Zero Trust approaches to networking',
                url: '#',
              },
            },
            {
              title: 'Automated Network Tasks',
              description:
                'Automate existing network infrastructure management tasks with a variety of Consul integrations. Cutdown on tickets for operators and speed up time to deployment for developers maintaining dynamic applications.',
              image: {
                url: require('./img/use-cases/network_automation.svg?url'),
                format: 'svg',
              },
              link: {
                title: 'Learn more about network infrastructure automation',
                url: '/use-cases/network-infrastructure-automation',
              },
            },
          ]}
        />
      </div>

      <CalloutBlade
        title="Get Started and Deploy Consul Service mesh for Kubernetes, VMs, or any environment"
        callouts={[
          {
            icon: require('./img/kubernetes/logo.svg?include'),
            title: 'Consul for Kubernetes',
            description:
              'Implement Consul service mesh on any Kubernetes distribution, connect between multiple Kubernetes clusters, and support traditional VM based applications with a single tool. Consul CRDs enable a self-service, Kubernetes native workflow for managing traffic patterns, permissions, and deployments of mesh applications.',
            eyebrow: 'Tutorial',
            link: {
              text: 'Install Consul on your Kubernetes cluster with Helm',
              url:
                'https://learn.hashicorp.com/tutorials/consul/kubernetes-custom-resource-definitions?in=consul/kubernetes',
            },
          },
          {
            icon: require('./img/kubernetes/communication-arrows.svg?include'),
            title: 'Consul for Everything Else',
            description:
              'Implement Consul service mesh and secure service-to-service communication across any runtime and across both public and private clouds. Use Consul service discovery and network infrastructure automation to avoid hard coding IPs, automate updates to network devices, and eliminate ticket based systems.',
            eyebrow: 'Tutorial',
            link: {
              text:
                'Get started with Consul as a service mesh for any VM-based environment',
              url:
                'https://learn.hashicorp.com/tutorials/consul/service-mesh-deploy-vms?in=consul/developer-mesh',
            },
          },
        ]}
      />
      <div className="ecosystem g-grid-container">
        <h2 className="g-type-display-2">Consul Ecosystem</h2>
        <TextSplitWithImage
          textSplit={{
            product: 'consul',
            heading: 'The Single Control Plane for Cloud Networks',
            content:
              'Consul provides the control plane for multi-cloud networking.',
            checkboxes: [
              'Centrally control the distributed data plane to provide a scalable and reliable service mesh',
              'Automate centralized network middleware configuration to avoid human intervention',
              'Provide a real-time directory of all running services to improve application inventory management',
              'Enable visibility into services and their health status to enhance health and performance monitoring',
              'Automate lifecycle management of certificates which can be issued by 3rd party Certificate Authority',
              'Provide unified support across a heterogeneous environment with different workload types and runtime platforms',
            ],
            linkStyle: 'links',
            links: [
              {
                type: 'outbound',
                text: 'Explore Consul Integrations',
                url: 'https://www.hashicorp.com/integrations/?filters=consul',
              },
            ],
          }}
          image={{
            url:
              'https://www.datocms-assets.com/2885/1622152328-control-plane.png',
            alt: 'Consul control plane',
          }}
        />
      </div>
      <section className="features">
        <div className="g-grid-container">
          <h3 className="g-type-display-2">Features</h3>
          <SteppedFeatureList
            features={[
              {
                title: 'Secure Service to Service Connectivity',
                description:
                  'Use mTLS to authenticate and secure connections between services.',
                learnMoreLink:
                  'https://learn.hashicorp.com/collections/consul/service-mesh-security',
                content: (
                  <img
                    src={require('../use-cases/img/multi-platform-service-mesh/service-to-service.png')}
                    alt="Service to Service Connectivity"
                  />
                ),
              },
              {
                title: 'Enhanced Observability',
                description:
                  'Visualize the service mesh topology with Consul’s built-in UI or third-party APM solutions.',
                learnMoreLink:
                  'https://learn.hashicorp.com/collections/consul/service-mesh-observability',
                content: (
                  <img
                    src={require('../use-cases/img/multi-platform-service-mesh/observability@3x.png')}
                    alt="Enhanced Observability"
                  />
                ),
              },
              {
                title: 'Layer 7 Traffic Management',
                description:
                  'Implement fine-grained traffic policies to route and split traffic across services.',
                learnMoreLink:
                  'https://learn.hashicorp.com/collections/consul/service-mesh-traffic-management',
                content: (
                  <CodeBlock
                    language="hcl"
                    code={`
                      Kind = "service-splitter"
                      Name = "web"
                      Splits = [
                        {
                          Weight        = 90
                          ServiceSubset = "v1"
                        },
                        {
                          Weight        = 10
                          ServiceSubset = "v2"
                        },
                      ]
                      `}
                  />
                ),
              },
              {
                title: 'Multi-platform Support',
                description:
                  'Consul service mesh can be deployed in any environment and supports multiple runtimes, like Kubernetes, Nomad, and VMs.',
                learnMoreLink:
                  'https://learn.hashicorp.com/collections/consul/gs-consul-service-mesh',
                content: (
                  <img
                    src={require('../use-cases/img/multi-platform-service-mesh/kubernetes-extend.png')}
                    alt="Multi-platform Support"
                  />
                ),
              },
            ]}
          />
        </div>
      </section>
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

      <LearnCallout
        headline="Learn the latest Consul skills"
        product="consul"
        background=""
        items={[
          {
            title: 'Service Mesh on Kubernetes',
            category: 'For Kubernetes',
            time: '3 hr 20 min',
            link: 'https://learn.hashicorp.com/collections/consul/kubernetes',
            image:
              'https://www.datocms-assets.com/2885/1600191254-hashicorp-icon.svg',
          },
          {
            title: 'HashiCorp Cloud Platform (HCP) Consul',
            category: 'Get Started',
            time: '59 mins',
            link:
              'https://learn.hashicorp.com/collections/consul/cloud-get-started',
            image:
              'https://www.datocms-assets.com/2885/1600191254-hashicorp-icon.svg',
          },
        ]}
      />
    </div>
  )
}
