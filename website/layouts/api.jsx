import DocsPage from '@hashicorp/react-docs-page'
import order from '../data/api-navigation.js'
import { frontMatter as data } from '../pages/api-docs/**/*.mdx'
import { MDXProvider } from '@mdx-js/react'
import EnterpriseAlert from '../components/enterprise-alert'
import Head from 'next/head'
import Link from 'next/link'

const DEFAULT_COMPONENTS = { EnterpriseAlert }

function ApiDocsLayoutWrapper(pageMeta) {
  function ApiDocsLayout(props) {
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
            category: 'api-docs',
            currentPage: props.path,
            data,
            order,
          }}
          resourceURL={`https://github.com/hashicorp/consul/blob/master/website/pages/${pageMeta.__resourcePath}`}
        />
      </MDXProvider>
    )
  }

  ApiDocsLayout.getInitialProps = ({ asPath }) => ({ path: asPath })

  return ApiDocsLayout
}

export default ApiDocsLayoutWrapper
