import UseCaseLayout from 'components/use-cases-layout'
import FeaturedSlider from '@hashicorp/react-featured-slider'
import TextSplitWithCode from '@hashicorp/react-text-split-with-code'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'

export default function ServiceDiscoveryAndHealthCheckingPage() {
  return (
    <UseCaseLayout
      title="Service Discovery and Health Checking"
      description="Discover, Register and Resolve services for application workloads across any cloud. Automatically add and remove services based on health checking."
      guideLink="https://learn.hashicorp.com/tutorials/consul/service-registration-health-checks"
    >
      <TextSplitWithImage
        textSplit={{
          heading: 'Centralized Service Registry',
          content:
            'Consul enables services to discover each other by storing location information (like IP addresses) in a single registry.',
          textSide: 'right',
          links: [
            {
              text: 'Learn More',
              url:
                'https://learn.hashicorp.com/consul/getting-started/services',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/discovery-health-checking/centralized-service-registry.png'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Real-time Health Monitoring',
          content:
            'Improve application resiliency by using Consul health checks to track the health of deployed services.',
          textSide: 'left',
          links: [
            {
              text: 'Learn More',
              url:
                'https://learn.hashicorp.com/consul/developer-discovery/health-checks',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: '/img/health-checking.png',
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Open and Extensible API',
          content:
            'Consul’s API allows users to integrate ecosystem technologies into their environments and enable service discovery at greater scale.',
          textSide: 'right',
          links: [
            {
              text: 'Learn More',
              url:
                'https://learn.hashicorp.com/consul?track=cloud-integrations#cloud-integrations',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/discovery-health-checking/extesnsible-api.png'),
        }}
      />

      <TextSplitWithCode
        textSplit={{
          heading: 'Simplified Resource Discovery',
          content:
            'Leverage DNS or HTTP interface to discover services and their locations registered with Consul.',
          textSide: 'left',
          links: [
            {
              text: 'Learn More',
              url:
                'https://learn.hashicorp.com/consul/getting-started/services',
              type: 'outbound',
            },
          ],
        }}
        codeBlock={{
          code: `$ dig @127.0.0.1 -p 8600 web.service.consul

; <<>> DiG 9.10.6 <<>> @127.0.0.1 -p 8600 web.service.consul SRV
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 56598
;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 3
;; WARNING: recursion requested but not available

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 4096
;; QUESTION SECTION:
;web.service.consul.    	IN  SRV

;; ANSWER SECTION:
web.service.consul. 0   IN  SRV 1 1 80 Judiths-MBP.lan.node.dc1.consul.

;; ADDITIONAL SECTION:
Judiths-MBP.lan.node.dc1.consul. 0 IN   A   127.0.0.1
Judiths-MBP.lan.node.dc1.consul. 0 IN   TXT "consul-network-segment="

;; Query time: 2 msec
;; SERVER: 127.0.0.1#8600(127.0.0.1)
;; WHEN: Tue Jul 16 14:31:13 PDT 2019
;; MSG SIZE  rcvd: 150`,
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Multi-Region, Multi-Cloud',
          content:
            'Consul’s distributed architecture allows it to be deployed at scale in any environment, in any region, on any cloud.',
          textSide: 'right',
          links: [
            {
              text: 'Learn More',
              url:
                'https://learn.hashicorp.com/collections/consul/datacenter-deploy#datacenter-deploy',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/discovery-health-checking/Map.png'),
        }}
      />

      <div className="with-border">
        <TextSplitWithImage
          textSplit={{
            heading: 'Built for Enterprise Scale',
            content:
              'Consul Enterprise provides the foundation for organizations to build a strong service networking platform at scale, with resiliency.',
            textSide: 'left',
            links: [
              {
                text: 'Read More',
                url: '/docs/enterprise',
                type: 'inbound',
              },
            ],
          }}
          image={{
            url: require('./img/discovery-health-checking/consul_screenshot@2x.png?url'),
          }}
        />
      </div>

      <FeaturedSlider
        heading="Case Study"
        theme="dark"
        features={[
          {
            logo: {
              url: require('./img/mercedes-logo.svg?url'),
              alt: 'Mercedes-Benz',
            },
            image: {
              url: require('./img/discovery-health-checking/mercedes-case-study.png'),
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
