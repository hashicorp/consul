import TextSplit from '@hashicorp/react-text-split'
import Button from '@hashicorp/react-button'
import s from './style.module.css'
import InlineSvg from '@hashicorp/react-inline-svg'
import ConsulStack from './img/consul-stack.svg?include'

export default function CtaHero({ title, description, links, cta }) {
  return (
    <div className={s.ctaHero}>
      <TextSplit
        product="consul"
        heading={title}
        content={description}
        links={links}
        linkStyle="buttons"
      >
        <CTA title={cta.title} description={cta.description} link={cta.link} />
      </TextSplit>
    </div>
  )
}

function CTA({ title, description, link }) {
  return (
    <div className={s.cta}>
      <InlineSvg className={s.stackIcon} src={ConsulStack} />
      <h3 className="g-type-display-3">{title}</h3>
      <p className={s.description}>{description}</p>
      <Button
        title={link.text}
        url={link.url}
        theme={{
          brand: 'neutral',
        }}
      />
    </div>
  )
}
