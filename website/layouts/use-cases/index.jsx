import BasicHero from '../../components/basic-hero'
import PrefooterCTA from '../../components/prefooter-cta'
import ConsulEnterpriseComparison from '../../components/enterprise-comparison/consul'
import Head from 'next/head'
import HashiHead from '@hashicorp/react-head'

export default function UseCaseLayout({ title, description, children }) {
  const pageTitle = `Consul ${title}`
  return (
    <>
      <HashiHead is={Head} title={pageTitle} description={description}>
        <meta property="og:title" content={pageTitle} />
      </HashiHead>

      <div id="p-use-case">
        <BasicHero
          heading={title}
          content={description}
          brand="consul"
          links={[
            {
              text: 'Explore HashiCorp Learn',
              url: 'https://learn.hashicorp.com/consul',
              type: 'outbound',
            },
            {
              text: 'Explore Documentation',
              url: '/docs',
              type: 'inbound',
            },
          ]}
        />
        <div className="g-grid-container">
          <h2 className="g-type-display-2 features-header">Features</h2>
        </div>
        {children}
        <ConsulEnterpriseComparison />
        <PrefooterCTA />
      </div>
    </>
  )
}
