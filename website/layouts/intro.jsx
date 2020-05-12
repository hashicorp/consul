import DocsPage from '@hashicorp/react-docs-page'
import order from '../data/intro-navigation.js'
import { frontMatter as data } from '../pages/intro/**/*.mdx'
import { MDXProvider } from '@mdx-js/react'
import EnterpriseAlert from '../components/enterprise-alert'
import Head from 'next/head'
import Link from 'next/link'

const DEFAULT_COMPONENTS = { EnterpriseAlert }

function IntroLayoutWrapper(pageMeta) {
  function IntroLayout(props) {
    return (
      <MDXProvider components={DEFAULT_COMPONENTS}>
        <DocsPage
          {...props}
          product="consul"
          head={{
            is: Head,
            title: `${pageMeta.page_title} | Consul by HashiCorp`,
            description: pageMeta.description,
            siteName: 'Consul by HashiCorp',
          }}
          sidenav={{
            Link,
            category: 'intro',
            currentPage: props.path,
            data,
            order,
          }}
          resourceURL={`https://github.com/hashicorp/consul/blob/master/website/pages/${pageMeta.__resourcePath}`}
        />
      </MDXProvider>
    )
  }

  IntroLayout.getInitialProps = ({ asPath }) => ({ path: asPath })

  return IntroLayout
}

export default IntroLayoutWrapper
