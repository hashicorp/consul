import Head from 'next/head'
import { blocks } from 'data/consul-on-kubernetes'
import BlockList from 'components/block-list'
import SideBySide from 'components/side-by-side'
import Button from '@hashicorp/react-button'

import s from './style.module.css'

export default function ConsulOnKubernetesPage() {
  return (
    <div>
      <Head>
        <title key="title">Consul on Kubernetes</title>
      </Head>

      {/* hero */}

      <section>
        <SideBySide
          left={
            <div>
              <h4 className={s.sideBySideTitle}>Overview</h4>
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
            </div>
          }
          right={
            <div>
              <h4 className={s.sideBySideTitle}>Challenges</h4>
              <BlockList blocks={blocks} />
            </div>
          }
        />
      </section>

      {/* features list section */}

      {/* get started section */}
    </div>
  )
}
