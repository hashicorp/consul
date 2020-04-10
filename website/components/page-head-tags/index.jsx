import Head from 'next/head'

export default function PageHeadTags() {
  const title = 'Consul by HashiCorp'
  const description =
    'Consul is a service networking solution to connect and secure services across any runtime platform and public or private cloud'
  const handle = '@HashiCorp'
  const tags = [
    { tag: 'title', content: title },
    {
      tag: 'meta',
      attributes: { property: 'og:title', content: title },
    },
    {
      tag: 'meta',
      attributes: { name: 'twitter:title', content: title },
    },
    {
      tag: 'meta',
      attributes: { property: 'description', content: description },
    },
    {
      tag: 'meta',
      attributes: { name: 'og:description', content: description },
    },
    {
      tag: 'meta',
      attributes: { name: 'twitter:card', content: 'summary_large_image' },
    },
    {
      tag: 'meta',
      attributes: { property: 'twitter:site', content: handle },
    },
    {
      tag: 'meta',
      attributes: { name: 'twitter:creator', content: handle },
    },
    {
      tag: 'meta',
      attributes: { name: 'og:type', content: 'website' },
    },
    {
      tag: 'meta',
      attributes: { name: 'og:url', content: 'https://www.consul.io/' },
    },
    {
      tag: 'meta',
      attributes: { name: 'og:site_name', content: title },
    },
    {
      tag: 'meta',
      attributes: {
        name: 'og:image',
        content: 'https://www.consul.io/assets/images/og-image-6ef0ad8b.png',
      },
    },
  ]
  return (
    <Head>
      {tags.map((item) => {
        if (item.tag === 'title') {
          return <title key="title">{item.content}</title>
        } else if (item.tag === 'meta') {
          return (
            <meta
              {...item.attributes}
              key={
                item.attributes.property
                  ? item.attributes.property
                  : item.attributes.name
              }
            />
          )
        }
      })}
    </Head>
  )
}
