import Head from 'next/head'
import Link from 'next/link'
import { createMdxProvider } from '@hashicorp/nextjs-scripts/lib/providers/docs'
import DocsPage from '@hashicorp/react-docs-page'
import { SearchProvider } from '@hashicorp/react-search'
import SearchBar from '../components/search-bar'
import { frontMatter as data } from '../pages/api-docs/**/*.mdx'
import order from '../data/api-navigation.js'

const MDXProvider = createMdxProvider({ product: 'consul' })

export default function ApiDocsLayout({
  frontMatter,
  path,
  children,
  ...propsWithoutChildren
}) {
  return (
    <MDXProvider>
      <DocsPage
        {...propsWithoutChildren}
        product="consul"
        head={{
          is: Head,
          title: `${frontMatter.page_title} | Consul by HashiCorp`,
          description: frontMatter.description,
          siteName: 'Consul by HashiCorp',
        }}
        sidenav={{
          Link,
          category: 'api-docs',
          currentPage: path,
          data,
          disableFilter: true,
          order,
        }}
        resourceURL={`https://github.com/hashicorp/consul/blob/master/website/pages/${frontMatter.__resourcePath}`}
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
