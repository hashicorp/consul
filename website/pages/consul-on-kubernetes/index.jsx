import Head from 'next/head'
import { features } from '../../data/consul-on-kubernetes'
import FeaturesList from '../../components/features-list'
// import s from './style.module.css'

export default function ConsulOnKubernetesPage() {
  return (
    <div>
      <Head>
        <title key="title">Consul on Kubernetes</title>
      </Head>

      {/* hero */}

      <section>
        <FeaturesList title="Why Consul on Kubernetes" features={features} />
      </section>

      {/* features list section */}

      {/* get started section */}
    </div>
  )
}
