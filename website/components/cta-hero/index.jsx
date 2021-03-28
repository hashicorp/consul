import Button from '@hashicorp/react-button'
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
  return (
    <div className={s.cta}>
      <img src={require('./img/cta-image.jpg?url')} alt="Consul stack" />
      <div className={s.ctaContent}>
        <div className={s.ctaHeading}>
          <h4 className="g-type-display-4">Try HCP Consul</h4>
        </div>
        <div className={s.ctaDescription}>
          <p className="g-type-body-small">
            Hosted on HashiCorp Cloud Platform, HCP Consul is a fully managed
            service mesh that discovers and securely connects any service.
          </p>
          <Button
            title="Sign Up"
            linkType="inbound"
            theme={{ variant: 'tertiary' }}
            url="https://consul.io"
          />
        </div>
      </div>
    </div>
  )
}
