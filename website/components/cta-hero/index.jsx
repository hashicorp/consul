import TextSplit from '@hashicorp/react-text-split'
import s from './style.module.css'

export default function CtaHero() {
  return (
    <div className={s.ctaHero}>
      <TextSplit
        heading="Service Mesh for any runtime or cloud"
        content="Consul automates networking for simple and secure application delivery."
        brand="consul"
        links={[
          { type: 'none', text: 'Download Consul', url: 'https://consul.io' },
          { type: 'none', text: 'Explore Tutorials', url: 'https://consul.io' },
        ]}
        linkStyle="buttons"
      >
        <Cta />
      </TextSplit>
    </div>
  )
}

function Cta() {
  return <div className={s.cta}>{'<Cta />'}</div>
}
