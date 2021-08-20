import VerticalTextBlockList from '@hashicorp/react-vertical-text-block-list'
import SectionHeader from '@hashicorp/react-section-header'
import Head from 'next/head'
import { productSlug } from 'data/metadata'
import s from './style.module.css'

export default function CommunityPage() {
  return (
    <div className={s.root}>
      <Head>
        <title key="title">Community | Consul by HashiCorp</title>
      </Head>
      <SectionHeader
        headline="Community"
        description="Consul is a large project with a growing community. There are active, dedicated users willing to help you through various mediums."
        use_h1={true}
      />
      <VerticalTextBlockList
        product={productSlug}
        data={[
          {
            header: 'Community Forum',
            body:
              '<a href="https://discuss.hashicorp.com/c/consul">Consul Community Forum</a>',
          },
          {
            header: 'Bug Tracker',
            body:
              '<a href="https://github.com/hashicorp/consul/issues">Issue tracker on GitHub</a>. Please only use this for reporting bugs. Do not ask for general help here; use Gitter or the mailing list for that.',
          },
          {
            header: 'Community Tools',
            body:
              '<a href="/docs/download-tools">Download Community Tools</a>. Please check out some of the awesome Consul tooling our amazing community has helped build.',
          },
          {
            header: 'Training',
            body:
              'Paid <a href="https://www.hashicorp.com/training">HashiCorp training courses</a> are also available in a city near you. Private training courses are also available.',
          },
          {
            header: 'Certification',
            body:
              'Learn more about our <a href="https://www.hashicorp.com/certification/">Cloud Engineer Certification program</a> and <a href="https://www.hashicorp.com/certification/consul-associate/">HashiCorp&apos;s Networking Automation Certification </a> exams.',
          },
        ]}
      />
    </div>
  )
}
