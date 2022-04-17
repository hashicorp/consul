import './style.css'
import '@hashicorp/platform-util/nprogress/style.css'

import useFathomAnalytics from '@hashicorp/platform-analytics'
import Router from 'next/router'
import Head from 'next/head'
import rivetQuery from '@hashicorp/platform-cms'
import NProgress from '@hashicorp/platform-util/nprogress'
import { ErrorBoundary } from '@hashicorp/platform-runtime-error-monitoring'
import createConsentManager from '@hashicorp/react-consent-manager/loader'
import localConsentManagerServices from 'lib/consent-manager-services'
import useAnchorLinkAnalytics from '@hashicorp/platform-util/anchor-link-analytics'
import HashiHead from '@hashicorp/react-head'
import AlertBanner from '@hashicorp/react-alert-banner'
import alertBannerData, { ALERT_BANNER_ACTIVE } from '../data/alert-banner'
import Error from './_error'
import StandardLayout from 'layouts/standard'

NProgress({ Router })
const { ConsentManager } = createConsentManager({
  preset: 'oss',
  otherServices: [...localConsentManagerServices],
})

export default function App({ Component, pageProps, layoutData }) {
  useFathomAnalytics()
  useAnchorLinkAnalytics()

  const Layout = Component.layout ?? StandardLayout

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
      <Layout {...(layoutData && { data: layoutData })}>
        <div className="content">
          <Component {...pageProps} />
        </div>
      </Layout>
      <ConsentManager />
    </ErrorBoundary>
  )
}

App.getInitialProps = async ({ Component, ctx }) => {
  const layoutQuery = Component.layout
    ? Component.layout?.rivetParams ?? null
    : StandardLayout.rivetParams

  const layoutData = layoutQuery ? await rivetQuery(layoutQuery) : null

  let pageProps = {}

  if (Component.getInitialProps) {
    pageProps = await Component.getInitialProps(ctx)
  }
  return { pageProps, layoutData }
}
