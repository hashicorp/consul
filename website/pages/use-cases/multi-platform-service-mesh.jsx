import UseCaseLayout from 'components/use-cases-layout'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'

export default function MultiPlatformServiceMeshPage() {
  return (
    <UseCaseLayout
      title="Multi-Platform Service Mesh"
      description="Create a consistent platform for modern application networking and security with identity based authorization, L7 traffic management, and service-to-service encryption."
      guideLink="https://learn.hashicorp.com/tutorials/consul/service-mesh-deploy?utm_source=WEBSITE&utm_medium=WEB_IO&utm_offer=ARTICLE_PAGE&utm_content=DOCS"
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
          url: require('./img/multi-platform-service-mesh/muilti-datacenter.png'),
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
                'https://learn.hashicorp.com/tutorials/consul/service-mesh-with-envoy-proxy',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/multi-platform-service-mesh/service-to-service.png'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Layer 7 Traffic Management',
          content:
            'Service-to-service communication policy at Layer 7 enables progressive delivery of application communication. Leverage Blue/Green or Canary deployment patterns for applications, enabling advanced traffic management patterns such as service failover, path-based routing, and traffic shifting that can be applied across public and private clouds, platforms, and networks.',
          textSide: 'right',
          links: [
            {
              text: 'Learn More',
              url:
                'https://learn.hashicorp.com/tutorials/consul/service-mesh-features',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/multi-platform-service-mesh/traffic_mgmt@3x.png?url'),
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
              url: '/docs/k8s/installation/install',
              type: 'inbound',
            },
          ],
        }}
        image={{
          url: require('./img/multi-platform-service-mesh/kubernetes-extend.png'),
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
          url: require('./img/multi-platform-service-mesh/extend-mesh.svg?url'),
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
          url: require('./img/multi-platform-service-mesh/Ecosystem.png'),
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Improved Observability',
          content:
            'Gain insight into service health and performance metrics with a built-in visualization directly in the Consul UI or by exporting metrics to a third-party solution.',
          textSide: 'right',
          links: [
            {
              text: 'Learn More',
              url: '/docs/agent/options#ui_config_metrics_provider',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/multi-platform-service-mesh/observability@3x.png?url'),
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
    </UseCaseLayout>
  )
}
