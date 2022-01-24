import VERSION from 'data/version'
import { productSlug } from 'data/metadata'
import ProductDownloadsPage from '@hashicorp/react-product-downloads-page'
import { generateStaticProps } from '@hashicorp/react-product-downloads-page/server'
import baseProps from 'components/downloads-props'

export default function DownloadsPage(staticProps) {
  return <ProductDownloadsPage {...baseProps()} {...staticProps} />
}

export async function getStaticProps() {
  return generateStaticProps({
    product: productSlug,
    latestVersion: VERSION,
  })
}
