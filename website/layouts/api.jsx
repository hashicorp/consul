import Head from 'next/head'
import Link from 'next/link'
import { createMdxProvider } from '@hashicorp/nextjs-scripts/lib/providers/docs'
import DocsPage from '@hashicorp/react-docs-page'
import { SearchProvider } from '@hashicorp/react-search'
import SearchBar from '../components/search-bar'
import { frontMatter as data } from '../pages/api-docs/**/*.mdx'
import order from '../data/api-navigation.js'

const MDXProvider = createMdxProvider({ product: 'consul' })

function ApiDocsLayoutWrapper(pageMeta) {
  function ApiDocsLayout(props) {
    const { children, ...propsWithoutChildren } = props
    return (
      <MDXProvider>
        <DocsPage
          {...propsWithoutChildren}
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
            disableFilter: true,
            order,
          }}
          resourceURL={`https://github.com/hashicorp/consul/blob/master/website/pages/${pageMeta.__resourcePath}`}
        >
          <SearchProvider>
            <SearchBar />
            {children}
          </SearchProvider>
        </DocsPage>
      </MDXProvider>
    )
  }

  ApiDocsLayout.getInitialProps = ({ asPath }) => ({ path: asPath })

  return ApiDocsLayout
}

export default ApiDocsLayoutWrapper
