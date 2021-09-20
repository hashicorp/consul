import Head from 'next/head'
import { features } from 'data/consul-on-kubernetes'
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
        <FeaturesList title="Why Consul on Kubernetes" features={features} />
      </section>

      {/* get started section */}
    </div>
  )
}
