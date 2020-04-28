import './style.css'
import App from 'next/app'
import NProgress from 'nprogress'
import Router from 'next/router'
import ProductSubnav from '../components/subnav'
import MegaNav from '@hashicorp/react-mega-nav'
import Footer from '../components/footer'
import AlertBanner from '@hashicorp/react-alert-banner'
import { ConsentManager, open } from '@hashicorp/react-consent-manager'
import consentManagerConfig from '../lib/consent-manager-config'
import bugsnagClient from '../lib/bugsnag'
import anchorLinkAnalytics from '../lib/anchor-link-analytics'
import alertBannerData, { ALERT_BANNER_ACTIVE } from '../data/alert-banner'
import Error from './_error'
import Head from 'next/head'
import HashiHead from '@hashicorp/react-head'

Router.events.on('routeChangeStart', NProgress.start)
Router.events.on('routeChangeError', NProgress.done)
Router.events.on('routeChangeComplete', (url) => {
  setTimeout(() => window.analytics.page(url), 0)
  NProgress.done()
})

// Bugsnag
const ErrorBoundary = bugsnagClient.getPlugin('react')

class NextApp extends App {
  static async getInitialProps({ Component, ctx }) {
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

  componentDidMount() {
    anchorLinkAnalytics()
  }

  componentDidUpdate() {
    anchorLinkAnalytics()
  }

  render() {
    const { Component, pageProps } = this.props

    return (
      <ErrorBoundary FallbackComponent={Error}>
        <HashiHead
          is={Head}
          title="Consul by HashiCorp"
          siteName="Consul by HashiCorp"
          description="Consul is a service networking solution to connect and secure services across any runtime platform and public or private cloud."
          image="https://www.consul.io/img/og-image.png"
          stylesheet={[
            { href: '/css/nprogress.css' },
            {
              href:
                'https://fonts.googleapis.com/css?family=Open+Sans:300,400,600,700&display=swap',
            },
          ]}
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
        />
        {ALERT_BANNER_ACTIVE && (
          <AlertBanner {...alertBannerData} theme="consul" />
        )}
        <MegaNav product="Consul" />
        <ProductSubnav />
        <div className="content">
          <Component {...pageProps} />
        </div>
        <Footer openConsentManager={open} />
        <ConsentManager {...consentManagerConfig} />
      </ErrorBoundary>
    )
  }
}

export default NextApp
