import VerticalTextBlockList from '@hashicorp/react-vertical-text-block-list'
import SectionHeader from '@hashicorp/react-section-header'
import Head from 'next/head'

export default function CommunityPage() {
  return (
    <div id="p-community">
      <Head>
        <title key="title">Community | Consul by HashiCorp</title>
      </Head>
      <SectionHeader
        headline="Community"
        description="Consul is a large project with a growing community. There are active, dedicated users willing to help you through various mediums."
        use_h1={true}
      />
      <VerticalTextBlockList
        data={[
          {
            header: 'Community Forum',
            body:
              '[Consul Community Forum](https://discuss.hashicorp.com/c/consul)',
          },
          {
            header: 'Bug Tracker',
            body:
              '[Issue tracker on GitHub](https://github.com/hashicorp/consul/issues). Please only use this for reporting bugs. Do not ask for general help here; use Gitter or the mailing list for that.',
          },
          {
            header: 'Community Tools',
            body:
              '[Download Community Tools](/downloads_tools). Please check out some of the awesome Consul tooling our amazing community has helped build.',
          },
          {
            header: 'Training',
            body:
              'Paid [HashiCorp training courses](https://www.hashicorp.com/training) are also available in a city near you. Private training courses are also available.',
          },
        ]}
      />
    </div>
  )
}
