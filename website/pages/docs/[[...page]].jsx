import { productName, productSlug } from 'data/metadata'
import DocsPage from '@hashicorp/react-docs-page'
import ConfigEntryReference from 'components/config-entry-reference'
// Imports below are only used server-side
/**
 * DEBT: short term patch for "hidden" docs-sidenav items.
 * See components/_temp-enable-hidden-pages for details.
 * Revert to importing from @hashicorp/react-docs-page/server
 * once https://app.asana.com/0/1100423001970639/1200197752405255/f
 * is complete.
 **/
import {
  generateStaticPaths,
  generateStaticProps,
} from 'components/_temp-enable-hidden-pages'

//  Configure the docs path
const additionalComponents = { ConfigEntryReference }
const baseRoute = 'docs'
const navDataFile = `data/${baseRoute}-nav-data.json`
const navDataFileHidden = `data/${baseRoute}-nav-data-hidden.json`
const localContentDir = `content/${baseRoute}`
const mainBranch = 'master'
const product = { name: productName, slug: productSlug }

export default function DocsLayout(props) {
  return (
    <DocsPage
      additionalComponents={additionalComponents}
      baseRoute={baseRoute}
      product={product}
      staticProps={props}
    />
  )
}

export async function getStaticPaths() {
  const paths = await generateStaticPaths({
    localContentDir,
    navDataFile,
    navDataFileHidden,
  })
  return { paths, fallback: false }
}

export async function getStaticProps({ params }) {
  const props = await generateStaticProps({
    additionalComponents,
    localContentDir,
    mainBranch,
    navDataFile,
    navDataFileHidden,
    params,
    product,
  })
  return { props }
}
