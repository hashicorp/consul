import Head from 'next/head'
import Button from '@hashicorp/react-button'
import ConsulOnKubernetesHero from 'components/consul-on-kubernetes-hero'
import FeaturesList from 'components/features-list'
import BlockList from 'components/block-list'
import SideBySide from 'components/side-by-side'
import DocsList from 'components/docs-list'
import s from './style.module.css'

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

      <section>
        <SideBySide
          left={
            <>
              <h2 className={s.sideBySideTitle}>Overview</h2>
              <p className={s.leftSideText}>
                Kubernetes and service mesh tend to go hand and hand.
                Organizations that adopt Kubernetes are looking for a way to
                automate, secure, and observe the connections between pods and
                clusters. Consul and Kubernetes provide a scalable and highly
                resilient platform for microservices. Consul supports any
                Kubernetes runtime including hosted solutions like EKS, AKS,
                GKE, and OpenShift.
                <br />
                <br />
                Need help managing Consul on AWS? HCP Consul support Amazon
                Elastic Kubernetes Service (EKS). Get started today.
              </p>
              <Button
                title="Learn More"
                url="#TODO"
                theme={{
                  brand: 'consul',
                }}
              />
            </>
          }
          right={
            <>
              <h2 className={s.sideBySideTitle}>Challenges</h2>
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
            </>
          }
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
      <section>
        {/* card list */}
        <DocsList
          title="Documentation"
          docs={[
            {
              icon: {
                src: require('./images/docs/helm-icon.svg'),
                alt: 'helm',
              },
              description:
                'Consul offers an official Helm chart for quickly deploying and upgrading Consul on Kubernetes.',
              cta: {
                text: 'Heml Docs',
                url: '#TODO',
              },
            },
            {
              icon: {
                src: require('@hashicorp/mktg-logos/product/terraform/logomark/color.svg'),
                alt: 'terraform',
              },
              description:
                'Use Consul’s Terraform provider for deploying and maintaining Consul agents across both Kubernetes and non-Kubernetes environments.',
              cta: {
                text: 'Terraform Provider',
                url: '#TODO',
              },
            },
          ]}
        />
      </section>
    </div>
  )
}
