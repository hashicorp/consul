import CallToAction from '@hashicorp/react-call-to-action'

export default function PrefooterCTA() {
  return (
    <CallToAction
      heading="Ready to get started?"
      content="Consul Open Source addresses the technical complexity of managing production services by providing a way to discover, automate, secure and connect applications and networking configurations across distributed infrastructure and clouds."
      product="consul"
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
  )
}
