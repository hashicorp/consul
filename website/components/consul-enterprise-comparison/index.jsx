import EnterpriseComparison from '../enterprise-comparison'

const technicalComplexity = {
  title: 'Technical Complexity',
  label: 'Open Source',
  imageUrl:
    'https://www.datocms-assets.com/2885/1579883486-complexity-basic.png',
  description:
    'Consul open source enables individuals to discover services and securely manage connections between them across cloud, on-premise, and hybrid environments.',
  link: {
    text: 'View Open Source Features',
    url: 'https://www.hashicorp.com/products/consul/pricing/',
    type: 'outbound',
  },
}

const organizationalComplexity = {
  title: 'Organizational Complexity',
  label: 'Enterprise',
  imageUrl:
    'https://www.datocms-assets.com/2885/1579883488-complexity-advanced.png',
  description:
    'Consul Enterprise provides the foundation for organizations to build and govern an enterprise-ready service networking environment for multiple teams across multiple clouds',
  link: {
    text: 'View Enterprise Features',
    url: 'https://www.hashicorp.com/products/consul/pricing/',
    type: 'outbound',
  },
}

export default function NomadEnterpriseInfo() {
  return (
    <EnterpriseComparison
      title="When to consider Consul Enterprise"
      itemOne={technicalComplexity}
      itemTwo={organizationalComplexity}
      brand="consul"
    />
  )
}
