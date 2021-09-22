import Head from 'next/head'
import BlockList from 'components/block-list'
import ConsulOnKubernetesHero from 'components/consul-on-kubernetes-hero'
import FeaturesList from 'components/features-list'

export default function ConsulOnKubernetesPage() {
  return (
    <div>
      <Head>
        <title key="title">Consul on Kubernetes</title>
      </Head>

      <ConsulOnKubernetesHero
        title="Consul on Kubernetes"
        description="A robust service mesh for discovering and securely connecting applications on Kubernetes."
        ctas={[
          { text: 'Get Started', url: '#TODO' },
          { text: 'Try HCP Consul', url: '#TODO' },
        ]}
        media={{
          type: 'image',
          source: require('./images/sample-video.png'),
          alt: 'sample image',
        }}
      />

      {/* side by side section */}
      {/* block list will be a node within the sidebyside section once that is complete */}
      <section>
        <BlockList
          blocks={[
            {
              title: 'Multi-Cluster environments',
              description:
                'Organizations typically prefer to utilize a more distributed model for Kubernetes deployments. Rather than maintain a single cluster, they connect multiple environments for testing, staging, and production purposes.',
              image: require('./images/blocks/multi-cluster.svg'),
            },
            {
              title: 'Connecting K8s to non-K8s',
              description:
                'Creating consistency when connecting Kubernetes to non-Kubernetes environments can be challenging, workflows need additional automation to accommodate many virtual machines or containers.',
              image: require('./images/blocks/connecting.svg'),
            },
            {
              title: 'Securing K8s networking',
              description:
                'Securing Kubernetes networking with multiple layers of network policies can be challenging. Policies can be handled at the application layer, container/OS or at the networking level. ',
              image: require('./images/blocks/securing.svg'),
            },
            {
              title: 'Kubernetes Monitoring',
              description:
                'Obtaining insights on what’s going on and the health of Kubernetes clusters can be complicated. In addition, security issues and vulnerabilities need to be properly tracked.  ',
              image: require('./images/blocks/monitoring.svg'),
            },
          ]}
        />
      </section>

      <section>
        <FeaturesList
          title="Why Consul on Kubernetes"
          features={[
            {
              title: 'Multi-Platform',
              subtitle:
                'Support both Kubernetes and non-Kubernetes workloads on any runtime',
              infoSections: [
                {
                  heading: 'Why it matters',
                  content: (
                    <p>
                      You can connect almost any application to any runtime.
                      Consul supports virtual machines and containers across
                      just about any platform.
                    </p>
                  ),
                },
                {
                  heading: 'Features',
                  content: (
                    <ul>
                      <li>Run thousands of nodes with low latency</li>
                      <li>Support any Kubernetes distribution</li>
                      <li>
                        Work across Kubernetes & non-Kubernetes Environments
                      </li>
                    </ul>
                  ),
                },
              ],
              cta: {
                text: 'Try It Now',
                url: '#TODO',
              },
              image: require('./images/features/multi-platform.svg'),
            },
            {
              title: 'Kube-native Workflow',
              subtitle:
                'Use Consul’s Custom Resource Definitions (CRDs) to interact with Kubernetes',
              infoSections: [
                {
                  heading: 'Why it matters',
                  content: (
                    <p>
                      Reduce Application deployment times using a workflows not
                      technologies approach and Kube native tools instead of
                      manual scripts
                    </p>
                  ),
                },
                {
                  heading: 'Features',
                  content: (
                    <ul>
                      <li>Layer 7 Traffic</li>
                      <li>Ingress/Egress through Gateways</li>
                      <li>Custom Resource Definitions</li>
                    </ul>
                  ),
                },
              ],
              cta: {
                text: 'Try It Now',
                url: '#TODO',
              },
              image: require('./images/features/workflow.svg'),
            },
            {
              title: 'Observable',
              subtitle:
                'Use built in UI and enable Kubernetes metrics via helm configuration',
              infoSections: [
                {
                  heading: 'Why it matters',
                  content: (
                    <p>
                      Provide enhanced observability using Kubernetes tools or
                      use third party solutions to monitor Kubernetes
                      performance
                    </p>
                  ),
                },
                {
                  heading: 'Features',
                  content: (
                    <ul>
                      <li>Built in UI metrics</li>
                      <li>APM integrations (Prometheus, Datadog, etc.)</li>
                    </ul>
                  ),
                },
              ],
              cta: {
                text: 'Try It Now',
                url: '#TODO',
              },
              image: require('./images/features/observable.svg'),
            },
            {
              title: 'Secure',
              subtitle:
                'Offload security concerns from applications based on application security policies. With HCP, security is enabled by default.',
              infoSections: [
                {
                  heading: 'Why it matters',
                  content: (
                    <p>
                      You can connect almost any application to any runtime.
                      Consul supports virtual machines and containers across
                      just about any platform.
                    </p>
                  ),
                },
                {
                  heading: 'Features',
                  content: (
                    <ul>
                      <li>
                        Encryption & Authorization (mTLS) using certificates for
                        service identity
                      </li>
                      <li>Access Controls (ACLs) & Namespaces</li>
                      <li>Automated Certificate Management & Rotation</li>
                    </ul>
                  ),
                },
              ],
              cta: {
                text: 'Try It Now',
                url: '#TODO',
              },
              image: require('./images/features/secure.svg'),
            },
          ]}
        />
      </section>

      {/* get started section */}
    </div>
  )
}
