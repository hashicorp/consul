import './style.css'
import '@hashicorp/nextjs-scripts/lib/nprogress/style.css'

import Router from 'next/router'
import Head from 'next/head'
import NProgress from '@hashicorp/nextjs-scripts/lib/nprogress'
import { ErrorBoundary } from '@hashicorp/nextjs-scripts/lib/bugsnag'
import createConsentManager from '@hashicorp/nextjs-scripts/lib/consent-manager'
import useAnchorLinkAnalytics from '@hashicorp/nextjs-scripts/lib/anchor-link-analytics'
import HashiHead from '@hashicorp/react-head'
import HashiStackMenu from '@hashicorp/react-hashi-stack-menu'
import AlertBanner from '@hashicorp/react-alert-banner'
import Footer from '../components/footer'
import ProductSubnav from '../components/subnav'
import alertBannerData, { ALERT_BANNER_ACTIVE } from '../data/alert-banner'
import Error from './_error'

NProgress({ Router })
const { ConsentManager, openConsentManager } = createConsentManager({
  preset: 'oss',
})

export default function App({ Component, pageProps }) {
  useAnchorLinkAnalytics()
  return (
    <ErrorBoundary FallbackComponent={Error}>
      <HashiHead
        is={Head}
        title="Consul by HashiCorp"
        siteName="Consul by HashiCorp"
        description="Consul is a service networking solution to automate network configurations, discover services, and enable secure connectivity across any cloud or runtime."
        image="https://www.consul.io/img/og-image.png"
        icon={[{ href: '/_favicon.ico' }]}
      >
        <meta
          name="og:title"
          property="og:title"
          content="Consul by HashiCorp"
        />
      </HashiHead>
      {ALERT_BANNER_ACTIVE && (
        <AlertBanner {...alertBannerData} product="consul" />
      )}
      <HashiStackMenu />
      <ProductSubnav />
      <div className="content">
        <Component {...pageProps} />
      </div>
      <Footer openConsentManager={openConsentManager} />
      <ConsentManager />
    </ErrorBoundary>
  )
}
