import { productName, productSlug } from 'data/metadata'
import DocsPage from '@hashicorp/react-docs-page'
// Imports below are only used server-side
import {
  generateStaticPaths,
  generateStaticProps,
} from '@hashicorp/react-docs-page/server'

//  Configure the docs path
const baseRoute = 'api-docs'
const navDataFile = `data/${baseRoute}-nav-data.json`
const localContentDir = `content/${baseRoute}`
const mainBranch = 'main'
const product = { name: productName, slug: productSlug }

export default function ApiDocsLayout(props) {
  return (
    <DocsPage baseRoute={baseRoute} product={product} staticProps={props} />
  )
}

export async function getStaticPaths() {
  const paths = await generateStaticPaths({
    localContentDir,
    navDataFile,
  })
  return { paths, fallback: false }
}

export async function getStaticProps({ params }) {
  const props = await generateStaticProps({
    localContentDir,
    mainBranch,
    navDataFile,
    params,
    product,
  })
  return { props }
}
