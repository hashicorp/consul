import Head from 'next/head'
import FeaturesList from 'components/features-list'

export default function ConsulOnKubernetesPage() {
  return (
    <div>
      <Head>
        <title key="title">Consul on Kubernetes</title>
      </Head>

      {/* hero */}

      {/* side by side section */}

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
              image: require('./images/multi-platform.svg'),
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
              image: require('./images/workflow.svg'),
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
              image: require('./images/observable.svg'),
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
              image: require('./images/secure.svg'),
            },
          ]}
        />
      </section>

      {/* get started section */}
    </div>
  )
}
