import './style.css'
import '@hashicorp/nextjs-scripts/lib/nprogress/style.css'

import Router from 'next/router'
import Head from 'next/head'
import NProgress from '@hashicorp/nextjs-scripts/lib/nprogress'
import { ErrorBoundary } from '@hashicorp/nextjs-scripts/lib/bugsnag'
import createConsentManager from '@hashicorp/nextjs-scripts/lib/consent-manager'
import useAnchorLinkAnalytics from '@hashicorp/nextjs-scripts/lib/anchor-link-analytics'
import HashiHead from '@hashicorp/react-head'
import MegaNav from '@hashicorp/react-mega-nav'
import AlertBanner from '@hashicorp/react-alert-banner'
import Footer from '../components/footer'
import ProductSubnav from '../components/subnav'
import alertBannerData, { ALERT_BANNER_ACTIVE } from '../data/alert-banner'
import Error from './_error'

NProgress({ Router })
const { ConsentManager, openConsentManager } = createConsentManager({
  preset: 'oss',
})

function App({ Component, pageProps }) {
  useAnchorLinkAnalytics()
  return (
    <ErrorBoundary FallbackComponent={Error}>
      <HashiHead
        is={Head}
        title="Consul by HashiCorp"
        siteName="Consul by HashiCorp"
        description="Consul is a service networking solution to automate network configurations, discover services, and enable secure connectivity across any cloud or runtime."
        image="https://www.consul.io/img/og-image.png"
        icon={[{ href: '/favicon.ico' }]}
        preload={[
          { href: '/fonts/klavika/medium.woff2', as: 'font' },
          { href: '/fonts/gilmer/light.woff2', as: 'font' },
          { href: '/fonts/gilmer/regular.woff2', as: 'font' },
          { href: '/fonts/gilmer/medium.woff2', as: 'font' },
          { href: '/fonts/gilmer/bold.woff2', as: 'font' },
          { href: '/fonts/metro-sans/book.woff2', as: 'font' },
          { href: '/fonts/metro-sans/regular.woff2', as: 'font' },
          { href: '/fonts/metro-sans/semi-bold.woff2', as: 'font' },
          { href: '/fonts/metro-sans/bold.woff2', as: 'font' },
          { href: '/fonts/dejavu/mono.woff2', as: 'font' },
        ]}
      >
        <meta
          name="og:title"
          property="og:title"
          content="Consul by HashiCorp"
        />
      </HashiHead>
      {ALERT_BANNER_ACTIVE && (
        <AlertBanner {...alertBannerData} theme="consul" />
      )}
      <MegaNav product="Consul" />
      <ProductSubnav />
      <div className="content">
        <Component {...pageProps} />
      </div>
      <Footer openConsentManager={openConsentManager} />
      <ConsentManager />
    </ErrorBoundary>
  )
}

App.getInitialProps = async function ({ Component, ctx }) {
  let pageProps = {}

  if (Component.getInitialProps) {
    pageProps = await Component.getInitialProps(ctx)
  } else if (Component.isMDXComponent) {
    // fix for https://github.com/mdx-js/mdx/issues/382
    const mdxLayoutComponent = Component({}).props.originalType
    if (mdxLayoutComponent.getInitialProps) {
      pageProps = await mdxLayoutComponent.getInitialProps(ctx)
    }
  }

  return { pageProps }
}

export default App
