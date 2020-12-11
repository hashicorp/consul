import DocsPage from '@hashicorp/react-docs-page'
import order from '../data/commands-navigation.js'
import { frontMatter as data } from '../pages/commands/**/*.mdx'
import Head from 'next/head'
import Link from 'next/link'
import { createMdxProvider } from '@hashicorp/nextjs-scripts/lib/providers/docs'

const MDXProvider = createMdxProvider({ product: 'consul' })
export default function CommandsLayout({ frontMatter, path, ...props }) {
  return (
    <MDXProvider>
      <DocsPage
        {...props}
        product="consul"
        head={{
          is: Head,
          title: `${frontMatter.page_title} | Consul by HashiCorp`,
          description: frontMatter.description,
          siteName: 'Consul by HashiCorp',
        }}
        sidenav={{
          Link,
          category: 'commands',
          currentPage: path,
          data,
          order,
        }}
        resourceURL={`https://github.com/hashicorp/consul/blob/master/website/pages/${frontMatter.__resourcePath}`}
      />
    </MDXProvider>
  )
}

CommandsLayout.getInitialProps = ({ asPath }) => ({ path: asPath })
