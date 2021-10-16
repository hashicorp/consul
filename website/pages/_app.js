import './style.css'
import '@hashicorp/platform-util/nprogress/style.css'

import useFathomAnalytics from '@hashicorp/platform-analytics'
import Router from 'next/router'
import Head from 'next/head'
import NProgress from '@hashicorp/platform-util/nprogress'
import { ErrorBoundary } from '@hashicorp/platform-runtime-error-monitoring'
import createConsentManager from '@hashicorp/react-consent-manager/loader'
import useAnchorLinkAnalytics from '@hashicorp/platform-util/anchor-link-analytics'
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
  useFathomAnalytics()
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
      >
        <meta
          name="og:title"
          property="og:title"
          content="Consul by HashiCorp"
        />
      </HashiHead>
      {ALERT_BANNER_ACTIVE && (
        <AlertBanner {...alertBannerData} product="consul" hideOnMobile />
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
