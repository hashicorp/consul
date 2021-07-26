import LearnCallout from '@hashicorp/react-learn-callout'
import SteppedFeatureList from '@hashicorp/react-stepped-feature-list'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'
import CodeBlock from '@hashicorp/react-code-block'
import UseCases from '@hashicorp/react-use-cases'
import CalloutBlade from 'components/callout-blade'
import CaseStudyCarousel from 'components/case-study-carousel'
import HomepageHero from 'components/homepage-hero'
import StaticDynamicDiagram from 'components/static-dynamic-diagram'
import highlightString from '@hashicorp/platform-code-highlighting/highlight-string'

export default function HomePage({ serviceMeshIngressGatewayCode }) {
  return (
    <div className="p-home">
      <HomepageHero
        alert={{
          url: 'https://www.hashicorp.com/blog/announcing-consul-1-10',
          text: 'Consul 1.10 Is Now Generally Available',
          tag: 'Blog',
        }}
        title="Service Mesh for any runtime or cloud"
        description="Consul automates networking for simple and secure application delivery."
        links={[
          {
            external: false,
            title: 'Try HCP Consul',
            url:
              'https://portal.cloud.hashicorp.com/sign-up?utm_source=docs&utm_content=consul_hero',
          },
          {
            external: false,
            title: 'Get Started',
            url: 'https://learn.hashicorp.com/consul',
          },
        ]}
        uiVideo={{
          name: 'UI',
          playbackRate: 2,
          srcType: 'mp4',
          url: 'https://www.datocms-assets.com/2885/1621637919-consul-ui.mp4',
        }}
        cliVideo={{
          name: 'CLI',
          playbackRate: 2,
          srcType: 'mp4',
          url: 'https://www.datocms-assets.com/2885/1621637930-consul-cli.mp4',
        }}
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
                'Simplify developer interactions, improve observability, and enable robust traffic management with Consul service mesh.',
              image: {
                url: require('./img/use-cases/service_mesh.svg?url'),
                format: 'svg',
              },
              link: {
                title: 'Service Mesh',
                url: '/use-cases/multi-platform-service-mesh',
              },
            },
            {
              title: 'Secure Service-to-Service Access',
              description:
                'Secure service access and communication across any network with identity-driven, time-based controls.',
              image: {
                url: require('./img/use-cases/discovery_health_checking.svg?url'), // @TODO - Consider a more specific icon
                format: 'svg',
              },
              link: {
                title: 'Zero Trust Networks',
                url: '/use-cases/service-discovery-and-health-checking',
              },
            },
            {
              title: 'Automated Networking Tasks',
              description:
                'Cut down on tickets for operators and speed up time to deployment of dynamic applications.',
              image: {
                url: require('./img/use-cases/network_automation.svg?url'),
                format: 'svg',
              },
              link: {
                title: 'Network Infrastructure Automation',
                url: '/use-cases/network-infrastructure-automation',
              },
            },
          ]}
        />
      </div>

      <CalloutBlade
        title="Deploy Consul Service mesh for Kubernetes, VMs, or any environment"
        callouts={[
          {
            icon: require('./img/kubernetes/logo.svg?include'),
            title: 'Consul for Kubernetes',
            description:
              "Consul service mesh works on any Kubernetes distribution, connects multiple clusters, and supports VM-based applications. Consul CRDs provide a self-service, Kubernetes native workflow to manage traffic patterns and permissions in the mesh.",
            eyebrow: 'Tutorial',
            link: {
              text: 'Get Started with Consul on Kubernetes',
              url:
                'https://learn.hashicorp.com/tutorials/consul/kubernetes-custom-resource-definitions?in=consul/kubernetes',
            },
          },
          {
            icon: require('./img/kubernetes/communication-arrows.svg?include'),
            title: 'Consul for Everything Else',
            description:
              "Consul service mesh support multiple orchestrators, like Nomad and Amazon ECS. Not using service mesh? Consul's service discovery and network infrastructure automation capabilities can help solve any service networking challenge.",
            eyebrow: 'Tutorial',
            link: {
              text: 'Get Started with Service Mesh on VMs',
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
                    src={require('./img/service-to-service-transparent.png')}
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
                  <img
                    src={require('./img/service-splitter@2x.png')}
                    alt="Layer 7 Traffic Management"
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
                  <center>
                    <img
                      style={{ maxWidth: '75%' }}
                      src={require('../use-cases/img/multi-platform-service-mesh/kubernetes-extend-transparent.png')}
                      alt="Multi-platform Support"
                    />
                  </center>
                ),
              },
              {
                title: 'Dynamic Load Balancing & Firewalling',
                description:
                  'Automate manual networking tasks and reduce the reliance on ticket-based systems.',
                learnMoreLink:
                  'https://learn.hashicorp.com/collections/consul/network-infrastructure-automation',
                content: (
                  <img
                    src={require('../use-cases/img/network-automation/load-balancing.png')}
                    alt="Load Balancing & Firewalling"
                  />
                ),
              },
              {
                title: 'Simple Cross Datacenter Networking',
                description:
                  'Use mesh gateways to connect services across datacenters with Consul service mesh.',
                learnMoreLink:
                  'https://learn.hashicorp.com/tutorials/consul/service-mesh-gateways?in=consul/developer-mesh',
                content: (
                  <img
                    src={require('./img/multi-datacenter-transparent.png')}
                    alt="Cross Datacenter Networking"
                  />
                ),
              },
              {
                title: 'Service Discovery & Real-time Health Checks',
                description:
                  'Keep track of the location information and health status of all applications.',
                learnMoreLink:
                  'https://learn.hashicorp.com/collections/consul/developer-discovery',
                content: (
                  <img
                    src={require('../use-cases/img/discovery-health-checking/service-discovery-and-health-checking.svg')}
                    alt="Service Discovery & Real-time Health Checks"
                  />
                ),
              },
              {
                title: 'Bridging Service Mesh with Traditional Networks',
                description:
                  'Interact with applications that reside outside of the mesh with Consul’s terminating gateways.',
                learnMoreLink:
                  'https://learn.hashicorp.com/tutorials/consul/terminating-gateways-connect-external-services',
                content: (
                  <img
                    src={require('./img/extend-mesh-transparent.png')}
                    alt="Service Mesh with Traditional Networks"
                  />
                ),
              },
              {
                title: 'Connect new services into Service Mesh',
                description:
                  'Enable external applications to securely connect with service inside of the mesh using Consul’s Ingress Gateway.',
                learnMoreLink:
                  'https://learn.hashicorp.com/tutorials/consul/service-mesh-ingress-gateways',
                content: (
                  <CodeBlock
                    language="yaml"
                    code={serviceMeshIngressGatewayCode}
                  />
                ),
              },
            ]}
          />
        </div>
      </section>
      <CalloutBlade
        title="Better Together: Consul and the HashiCorp Stack"
        callouts={[
          {
            title: 'Automated Infrastructure with Terraform',
            icon: require('./img/stack/consul-and-terraform.svg?include'),
            description:
              'Speed up time to delivery for services with network infrastructure automation. Use Consul as a single source of truth for all services and apply configuration changes with Terraform.',
            link: {
              text: 'Consul with Terraform',
              url:
                'https://learn.hashicorp.com/tutorials/consul/consul-terraform-sync-intro?in=consul/network-infrastructure-automation',
            },
          },
          {
            title: 'Defense in Depth with Vault',
            icon: require('./img/stack/consul-and-vault.svg?include'),
            description:
              'Ensure complete security for service-to-service access, authorization and communication by using Consul and Vault. Deliver end-to-end authentication, authorization, and encryption using identity-based access controls and traffic policies for microservice architectures.              ',
            link: {
              text: 'Consul with Vault',
              url:
                'https://learn.hashicorp.com/collections/consul/vault-secure',
            },
          },
          {
            title: 'Application Delivery with Nomad',
            icon: require('./img/stack/consul-and-nomad.svg?include'),
            description:
              'Accelerate the application delivery lifecycle with orchestration and scheduling from Nomad and Consul service mesh. Enable developers to deploy and connect workloads in any environment with fewer code changes.',
            link: {
              text: 'Consul with Nomad',
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

export async function getStaticProps() {
  const rawYaml = `
  apiVersion: consul.hashicorp.com/v1alpha1
  kind: IngressGateway
  metadata:
    name: ingress-gateway
  spec:
    listeners:
      - port: 8080
        protocol: http
        services:
          - name: static-server
  `
  const serviceMeshIngressGatewayCode = await highlightString(rawYaml, 'yaml')
  return {
    props: {
      serviceMeshIngressGatewayCode,
    },
  }
}
