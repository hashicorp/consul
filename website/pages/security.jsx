import MarkdownPage from 'components/_temporary-markdown-page'
import generateStaticProps from 'components/_temporary-markdown-page/server'

export default function CommunityToolsPage({ staticProps }) {
  return <MarkdownPage {...staticProps} />
}

export const getStaticProps = generateStaticProps({
  pagePath: 'content/community-plugins.mdx',
})
