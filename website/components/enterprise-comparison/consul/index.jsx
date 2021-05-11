import EnterpriseComparison from '../../enterprise-comparison'

export default function ConsulEnterpriseComparison() {
  return (
    <EnterpriseComparison
      title="When to consider Consul Enterprise"
      itemOne={{
        title: 'Technical Complexity',
        label: 'Open Source',
        imageUrl: require('./img/enterprise_complexity_1.svg?url'),
        description:
          'Consul Open Source enables individuals to discover services and securely manage connections between them across cloud, on-prem, and hybrid environments.',
        links: [
          {
            text: 'View Open Source Features',
            url: 'https://www.hashicorp.com/products/consul/pricing/',
            type: 'outbound',
          },
        ],
      }}
      itemTwo={{
        title: 'Organizational Complexity',
        label: 'Enterprise',
        imageUrl: require('./img/enterprise_complexity_2.svg?url'),
        description:
          'Consul Enterprise provides the foundation for organizations to build an enterprise-ready service networking environment for multiple teams by enabling governance capabilities.',
        links: [
          {
            text: 'View Cloud Features',
            url:
              'https://cloud.hashicorp.com/?utm_source=consul_io&utm_content=ent_comparison',
            type: 'outbound',
          },
          {
            text: 'View Self-Managed Features',
            url: 'https://www.hashicorp.com/products/consul/pricing/',
            type: 'outbound',
          },
        ],
      }}
      brand="consul"
    />
  )
}
