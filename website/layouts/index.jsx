import DocsPage from '@hashicorp/react-docs-page'
import Head from 'next/head'
import Link from 'next/link'

export default function NoSidebarLayout({ frontMatter, path, ...props }) {
  return (
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
        category: 'docs',
        currentPage: path,
        data: [],
        order: [],
        disableFilter: true,
      }}
      resourceURL={`https://github.com/hashicorp/consul/blob/master/website/pages/${frontMatter.__resourcePath}`}
    />
  )
}

NoSidebarLayout.getInitialProps = ({ asPath }) => ({ path: asPath })
