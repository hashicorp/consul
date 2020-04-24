import CallToAction from '@hashicorp/react-call-to-action'
import UseCases from '@hashicorp/react-use-cases'
import BasicHero from '../../components/basic-hero'
import ConsulEnterpriseComparison from '../../components/consul-enterprise-comparison'
import LearnCallout from '../../components/learn-callout'

export default function HomePage() {
  return (
    <div className="p-home">
      <BasicHero
        brand="consul"
        heading="Service Networking Across Any Cloud or Runtime"
        content="Automate network configurations, discover services, and enable secure connectivity across any cloud or runtime"
        links={[
          {
            text: 'Explore HashiCorp Learn',
            url: 'https://learn.hashicorp.com/nomad',
            type: 'outbound',
          },
          {
            text: 'Explore Documentation',
            url: '/docs',
            type: 'inbound',
          },
        ]}
        backgroundImage
      />
      <div className="use-cases g-grid-container">
        <UseCases
          items={[
            {
              title: 'Infrastructure as Code',
              description:
                'Use infrastructure as code to provision infrastructure. Codification enables version control and automation, reducing human error and increasing productivity.',
              image: {
                url:
                  'https://www.datocms-assets.com/2885/1538425176-secrets.svg',
                alt: 'optional image',
                format: 'svg',
              },
              link: {
                title: 'Learn more',
                url: 'https://hashicorp.com',
              },
            },
            {
              title: 'Multi-Cloud Compliance and Management',
              description:
                'Provision and manage public cloud, private infrastructure, and cloud services with one workflow to learn, secure, govern, and audit.',
              image: {
                url:
                  'https://www.datocms-assets.com/2885/1538425176-secrets.svg',
                alt: 'optional image',
                format: 'svg',
              },
              link: {
                title: 'Learn more',
                url: 'https://hashicorp.com/products/terraform',
              },
            },
            {
              title: 'Self-Service Infrastructure',
              description:
                'Enable users to easily provision infrastructure on-demand with a library of approved infrastructure.',
              image: {
                url:
                  'https://www.datocms-assets.com/2885/1538425176-secrets.svg',
                alt: 'optional image',
                format: 'svg',
              },
              link: {
                title: 'Learn more',
                url: 'https://terraform.io',
                external: true,
              },
            },
          ]}
        />
      </div>
      <ConsulEnterpriseComparison />
      <LearnCallout
        headline="Learn the latest Consul skills"
        brand="consul"
        items={[
          {
            title: 'Getting Started',
            category: 'Step-by-Step Guides',
            time: '24 mins',
            link:
              'https://learn.hashicorp.com/nomad?track=getting-started#getting-started',
            image: 'https://www.datocms-assets.com/2885/1538425176-secrets.svg',
          },
          {
            title: 'Deploy and Manage Nomad Jobs',
            category: 'Step-by-Step Guides',
            time: '36 mins',
            link:
              'https://learn.hashicorp.com/nomad?track=managing-jobs#getting-started',
            image: 'https://www.datocms-assets.com/2885/1538425176-secrets.svg',
          },
        ]}
      />

      <CallToAction
        heading="Ready to get started?"
        content="Consul open source addresses the technical complexity of managing production services by providing a way to discover, secure, automate and connect applications and networking configurations across distributed infrastructure and clouds."
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
        variant="compact"
        theme="light"
      />
    </div>
  )
}
