import Head from 'next/head'
import { blocks } from 'data/consul-on-kubernetes'
import BlockList from 'components/block-list'

export default function ConsulOnKubernetesPage() {
  return (
    <div>
      <Head>
        <title key="title">Consul on Kubernetes</title>
      </Head>
      {/* hero */}

      {/* side by side section */}
      <section className="g-grid-container">
        <BlockList
          blocks={[
            {
              title: 'Multi-Cluster environments',
              description:
                'Organizations typically prefer to utilize a more distributed model for Kubernetes deployments. Rather than maintain a single cluster, they connect multiple environments for testing, staging, and production purposes.',
              image: '/img/consul-k8/blocks/multi-cluster.svg',
            },
            {
              title: 'Connecting K8s to non-K8s',
              description:
                'Creating consistency when connecting Kubernetes to non-Kubernetes environments can be challenging, workflows need additional automation to accommodate many virtual machines or containers.',
              image: '/img/consul-k8/blocks/connecting.svg',
            },
            {
              title: 'Securing K8s networking',
              description:
                'Securing Kubernetes networking with multiple layers of network policies can be challenging. Policies can be handled at the application layer, container/OS or at the networking level. ',
              image: '/img/consul-k8/blocks/securing.svg',
            },
            {
              title: 'Kubernetes Monitoring',
              description:
                'Obtaining insights on what’s going on and the health of Kubernetes clusters can be complicated. In addition, security issues and vulnerabilities need to be properly tracked.  ',
              image: '/img/consul-k8/blocks/monitoring.svg',
            },
          ]}
        />
      </section>

      {/* features list section */}

      {/* get started section */}
    </div>
  )
}
