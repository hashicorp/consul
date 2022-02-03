import query from './query.graphql'
import ProductSubnav from 'components/subnav'
import Footer from 'components/footer'
import { open } from '@hashicorp/react-consent-manager'

export default function StandardLayout(props: Props): React.ReactElement {
  const { useCaseNavItems } = props.data

  return (
    <>
      <ProductSubnav
        menuItems={[
          { text: 'Overview', url: '/' },
          {
            text: 'Use Cases',
            submenu: [
              { text: 'Consul on Kubernetes', url: '/consul-on-kubernetes' },
              ...useCaseNavItems.map((item) => {
                return {
                  text: item.text,
                  url: `/use-cases/${item.url}`,
                }
              }),
            ].sort((a, b) => a.text.localeCompare(b.text)),
          },
          {
            text: 'Enterprise',
            url:
              'https://www.hashicorp.com/products/consul/?utm_source=oss&utm_medium=header-nav&utm_campaign=consul',
            type: 'outbound',
          },
          'divider',
          {
            text: 'Tutorials',
            url: 'https://learn.hashicorp.com/consul',
            type: 'outbound',
          },
          {
            text: 'Docs',
            url: '/docs',
            type: 'inbound',
          },
          {
            text: 'API',
            url: '/api-docs',
            type: 'inbound',
          },
          {
            text: 'CLI',
            url: '/commands',
            type: 'inbound,',
          },
          {
            text: 'Community',
            url: '/community',
            type: 'inbound',
          },
        ]}
      />
      {props.children}
      <Footer openConsentManager={open} />
    </>
  )
}

StandardLayout.rivetParams = {
  query,
  dependencies: [],
}

interface Props {
  children: React.ReactChildren
  data: {
    useCaseNavItems: Array<{ url: string; text: string }>
  }
}
