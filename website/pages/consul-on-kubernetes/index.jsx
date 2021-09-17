import Head from 'next/head'
import CKHero from 'components/ck-hero'

export default function ConsulOnKubernetesPage() {
  return (
    <div>
      <Head>
        <title key="title">Consul on Kubernetes</title>
      </Head>
      <CKHero
        title="Consul on Kubernetes"
        subtitle="A robust service mesh for discovering and securely connecting applications on Kubernetes."
        ctas={[
          { text: 'Get Started', url: '#TODO' },
          { text: 'Try HCP Consul', url: '#TODO' },
        ]}
        media={{
          type: 'image',
          source: '/img/sample-video.png',
          alt: 'sample image',
        }}
      />
      {/* side by side section */}

      {/* features list section */}

      {/* get started section */}
    </div>
  )
}
