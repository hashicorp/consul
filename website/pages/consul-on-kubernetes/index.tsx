import ReactHead from '@hashicorp/react-head'
import Button from '@hashicorp/react-button'
import ConsulOnKubernetesHero from 'components/consul-on-kubernetes-hero'
import FeaturesList from 'components/features-list'
import BlockList from 'components/block-list'
import SideBySide from 'components/side-by-side'
import CardList from 'components/card-list'
import DocsList from 'components/docs-list'
import s from './style.module.css'

export default function ConsulOnKubernetesPage() {
  const pageDescription =
    'Consul is a robust service mesh for discovering and securely connecting applications on Kubernetes.'
  const pageTitle = 'Consul on Kubernetes'

  return (
    <div>
      <ReactHead
        title={pageTitle}
        pageName={pageTitle}
        description={pageDescription}
        image="/img/consul-on-kubernetes-share-image.png"
        twitterCard="summary_large_image"
      >
        <meta name="og:title" property="og:title" content={pageTitle} />
        <meta name="twitter:title" content={pageTitle} />
        <meta name="twitter:description" content={pageDescription} />
        <meta name="author" content="@HashiCorp" />
      </ReactHead>

      <ConsulOnKubernetesHero
        title="Consul on Kubernetes"
        description="A robust service mesh for discovering and securely connecting applications on Kubernetes."
        ctas={[
          {
            text: 'Try HCP Consul',
            url:
              'https://portal.cloud.hashicorp.com/?utm_source=docs&utm_content=consul_on_kubernetes_landing',
          },
          {
            text: 'Install Consul on Kubernetes',
            url: '/docs/k8s/installation/install',
          },
        ]}
        video={{
          src: 'https://www.youtube.com/watch?v=Eyszp_apaEU',
          poster: require('./images/hero/poster.png'),
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
                Need help managing Consul on AWS? HCP Consul supports Amazon
                Elastic Kubernetes Service (EKS). Get started today.
              </p>
              <Button
                title="Install Consul on Kubernetes"
                url="/docs/k8s/installation/install"
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
                    title: 'Multi-cluster',
                    description:
                      'Organizations typically prefer to utilize a more distributed model for Kubernetes deployments. Rather than maintain a single cluster, they connect multiple environments for testing, staging, and production purposes.',
                    image: require('./images/blocks/multi-cluster.svg'),
                  },
                  {
                    title: 'Connecting Kubernetes to non-Kubernetes',
                    description:
                      'Creating consistency when connecting Kubernetes to non-Kubernetes environments can be challenging, workflows need additional automation to accommodate many virtual machines or containers.',
                    image: require('./images/blocks/connecting.svg'),
                  },
                  {
                    title: 'Securing Kubernetes networking',
                    description:
                      'Securing Kubernetes networking with multiple layers of network policies can be challenging. Organizations need to apply policies at both the application layer and network layer to ensure consistent security.',
                    image: require('./images/blocks/securing.svg'),
                  },
                  {
                    title: 'Kubernetes monitoring',
                    description:
                      "Obtaining insights into what's happening inside the cluster and the overall health of the cluster. In addition, security issues and vulnerabilities need to be properly tracked.",
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
              title: 'Multi-platform',
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
                text: 'Start tutorial',
                url:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-deployment-guide?in=consul/kubernetes',
              },
              image: require('./images/features/multi-platform.svg'),
            },
            {
              title: 'Kube-native workflow',
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
                text: 'Start tutorial',
                url:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-custom-resource-definitions?in=consul/kubernetes',
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
                text: 'Start tutorial',
                url:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-layer7-observability?in=consul/kubernetes',
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
                text: 'Start tutorial',
                url:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-secure-agents?in=consul/kubernetes',
              },
              image: require('./images/features/secure.svg'),
            },
          ]}
        />
      </section>
      <section className={s.getStartedWrapper}>
        <h1 className={s.getStartedTitle}>Ways to get started</h1>
        <div className={s.getStartedContent}>
          <CardList
            title="Tutorials"
            className={s.getStartedCards}
            cards={[
              {
                eyebrow: '15 min',
                heading: 'Get started on Kubernetes',
                description:
                  'Setup Consul service mesh to get experience deploying service sidecar proxies and securing service with mTLS.',
                url:
                  'https://learn.hashicorp.com/tutorials/consul/service-mesh-deploy?in=consul/gs-consul-service-mesh',
              },
              {
                eyebrow: '22 min',
                heading: 'Secure Consul and registered services on Kubernetes',
                description:
                  'Secure Consul on Kubernetes using gossip encryption, TLS certificates, Access Control Lists, and Consul intentions.',
                url:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-secure-agents?in=consul/kubernetes',
              },
              {
                eyebrow: '21 min',
                heading:
                  'Layer 7 observability with Prometheus, Grafana, and Kubernetes',
                description:
                  'Collect and visualize layer 7 metrics from services in your Kubernetes cluster using Consul service mesh, Prometheus, and Grafana.',
                url:
                  'https://learn.hashicorp.com/tutorials/consul/kubernetes-layer7-observability?in=consul/kubernetes',
              },
            ]}
          />
          <DocsList
            title="Documentation"
            className={s.getStartedDocs}
            docs={[
              {
                icon: {
                  src: require('./images/docs/helm-icon.svg'),
                  alt: 'helm',
                },
                description:
                  'Consul offers an official Helm chart for quickly deploying and upgrading Consul on Kubernetes.',
                cta: {
                  text: 'Helm documentation',
                  url: '/docs/k8s/installation/install',
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
                  text: 'Terraform provider',
                  url:
                    'https://registry.terraform.io/providers/hashicorp/consul/latest/docs',
                },
              },
            ]}
          />
        </div>
      </section>
    </div>
  )
}
