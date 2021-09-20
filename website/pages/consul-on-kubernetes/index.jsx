import Head from 'next/head'
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
        <FeaturesList
          title="Why Consul on Kubernetes"
          features={[
            {
              title: '',
              subtitle: '',
              infoSections: [
                {
                  title: '',
                  content: '',
                },
              ],
              cta: {
                text: '',
                url: '',
              },
              image: '',
            },
          ]}
        />
      </section>

      {/* features list section */}

      {/* get started section */}
    </div>
  )
}
