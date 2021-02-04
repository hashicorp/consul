import { productName, productSlug } from 'data/metadata'
import order from 'data/docs-navigation.js'
import DocsPage from '@hashicorp/react-docs-page'
import {
  generateStaticPaths,
  generateStaticProps,
} from '@hashicorp/react-docs-page/server'
import ConfigEntryReference from 'components/config-entry-reference'

import Button from '@hashicorp/react-button'
import VideoModalCta from '../../components/introduction'
import Card from '../../components/card'
import Cards from '../../components/cards'

const subpath = 'docs'
const additionalComponents = {
  ConfigEntryReference,
  Button,
  VideoModalCta,
  Card,
  Cards,
}

export default function DocsLayout(props) {
  return (
    <DocsPage
      product={{ name: productName, slug: productSlug }}
      subpath={subpath}
      order={order}
      staticProps={props}
      mainBranch="master"
      additionalComponents={additionalComponents}
    />
  )
}

export async function getStaticPaths() {
  return generateStaticPaths(subpath)
}

export async function getStaticProps({ params }) {
  return generateStaticProps({
    subpath,
    productName,
    params,
    additionalComponents,
  })
}
