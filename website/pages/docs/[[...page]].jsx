import { productName, productSlug } from 'data/metadata'
import DocsPage from '@hashicorp/react-docs-page'
import ConfigEntryReference from 'components/config-entry-reference'
// Imports below are only used server-side
import {
  generateStaticPaths,
  generateStaticProps,
} from '@hashicorp/react-docs-page/server'

//  Configure the docs path
const additionalComponents = { ConfigEntryReference }
const baseRoute = 'docs'
const navDataFile = `data/${baseRoute}-nav-data.json`
const localContentDir = `content/${baseRoute}`
const mainBranch = 'main'
const product = { name: productName, slug: productSlug }

export default function DocsLayout(props) {
  return (
    <DocsPage
      additionalComponents={additionalComponents}
      baseRoute={baseRoute}
      product={product}
      staticProps={props}
      showVersionSelect={true}
    />
  )
}

export async function getStaticPaths() {
  const paths = await generateStaticPaths({
    navDataFile,
    localContentDir,
    // new ----
    product: { name: productName, slug: productSlug },
    basePath: baseRoute,
  })
  return { paths, fallback: 'blocking' }
}

export async function getStaticProps({ params }) {
  try {
    const props = await generateStaticProps({
      localContentDir,
      mainBranch,
      navDataFile,
      params,
      product,
      basePath: baseRoute,
    })
    return { props, revalidate: 10 }
  } catch (err) {
    console.warn(err)
    return {
      notFound: true,
    }
  }
}
