import UseCaseLayout from '../../layouts/use-cases'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'
import TextSplitWithCode from '@hashicorp/react-text-split-with-code'

export default function MultiPlatformServiceMeshPage() {
  return (
    <UseCaseLayout
      title="Multi-Platform Service Mesh"
      description="Create a consistent platform for modern application networking and security with identity based authorization, L7 traffic management, and service-to-service encryption."
      guideLink="https://learn.hashicorp.com/consul/gs-consul-service-mesh/understand-consul-service-mesh"
    >
      <TextSplitWithImage
        textSplit={{
          heading: 'Multi-Datacenter, Multi-Region',
          content:
            'Federate Consul between multiple clusters and environments creating a global service mesh. Consistently apply policies and security across platforms.',
          textSide: 'right',
          links: [
            {
              text: 'Learn More',
              url:
                'https://learn.hashicorp.com/consul/security-networking/datacenters',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/multi-dc-multi-region.svg?url'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Secure Service-to-Service Communication',
          content:
            'Automatic mTLS communication between services both inside Kubernetes and in traditional runtime platforms. Extend and integrate with external certificate platforms like Vault.',
          textSide: 'left',
          links: [
            {
              text: 'Learn More',
              url:
                'https://learn.hashicorp.com/consul/security-networking/certificates',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/service-to-service.svg?url'),
        }}
      />

      <TextSplitWithCode
        textSplit={{
          heading: 'Layer 7 Traffic Management',
          content:
            'Service-to-service communication policy at Layer 7 enables progressive delivery of application communication. Leverage Blue/Green or Canary deployment patterns for applications, enabling advanced traffic management patterns such as service failover, path-based routing, and traffic shifting that can be applied across public and private clouds, platforms, and networks.',
          textSide: 'right',
          links: [
            {
              text: 'Learn More',
              url:
                'https://www.consul.io/docs/connect/l7-traffic-management.html',
              type: 'outbound',
            },
          ],
        }}
        codeBlock={{
          language: 'hcl',
          code: `Kind = "service-splitter"
Name = "billing-api"

Splits = [
  {
    Weight        = 10
    ServiceSubset = "v2"
  },
  {
    Weight        = 90
    ServiceSubset = "v1"
  },
]`,
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Integrate and Extend in Kubernetes',
          content:
            'Quickly deploy Consul on Kubernetes leveraging Helm. Automatically inject sidecars for Kubernetes resources. Federate multiple clusters into a single service mesh.',
          textSide: 'left',
          links: [
            {
              text: 'Learn More',
              url: 'https://www.consul.io/docs/platform/k8s/run.html',
              type: 'inbound',
            },
          ],
        }}
        image={{
          url: require('./img/kubernetes.svg?url'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Connect and Extend Service Mesh for Any Workload',
          content:
            'Integrate with existing services and applications by leveraging Consul ingress and terminating gateways. Extend between complex networks and multi-cloud topologies with Consul mesh gateways.',
          textSide: 'right',
        }}
        image={{
          url: require('./img/connect-and-extend.svg?url'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Robust Ecosystem',
          content:
            'Rich ecosystem community extends Consul’s functionality into spaces such as networking, observability, and security.',
          textSide: 'left',
        }}
        image={{
          url: require('./img/robust-ecosystem.svg?url'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Improved Observability',
          content:
            'Centrally managed service observability at Layer 7 including detailed metrics on all service-to-service communication such as connections, bytes transferred, retries, timeouts, open circuits, and request rates, response codes.',
          textSide: 'right',
        }}
        image={{
          url: require('./img/observability.svg?url'),
        }}
      />

      <div className="with-border">
        <TextSplitWithImage
          textSplit={{
            heading: 'Scale to Enterprise',
            content:
              'Consul addresses the challenge of running a service mesh at enterprise scale from both an environmental complexity and resiliency perspective.',
            textSide: 'left',
            links: [
              {
                text: 'Learn More',
                url: 'https://www.consul.io/docs/enterprise/index.html',
                type: 'inbound',
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
