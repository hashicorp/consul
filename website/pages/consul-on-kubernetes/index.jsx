import Head from 'next/head'
import { blocks } from '/data/consul-on-kubernetes'
import BlockList from '/components/block-list'

export default function ConsulOnKubernetesPage() {
  return (
    <div>
      <Head>
        <title key="title">Consul on Kubernetes</title>
      </Head>
      {/* hero */}

      {/* side by side section */}
      <BlockList blocks={blocks} />

      {/* features list section */}

      {/* get started section */}
    </div>
  )
}
