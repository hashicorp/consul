import GlossaryPage from '@hashicorp/react-glossary-page'
import generateStaticProps from '@hashicorp/react-glossary-page/server'

import order from 'data/docs-navigation.js'
import { productName, productSlug } from 'data/metadata'

export default function GlossaryLayout({ terms, content, docsPageData }) {
  return (
    <GlossaryPage
      content={content}
      docsPageData={docsPageData}
      mainBranch="master"
      order={order}
      product={{ name: productName, slug: productSlug }}
      terms={terms}
      staticProps={{ terms, content, docsPageData }}
    />
  )
}

export async function getStaticProps() {
  return generateStaticProps({
    productName,
    subpath: 'docs',
  })
}
