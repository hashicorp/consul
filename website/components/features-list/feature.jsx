import s from './style.module.css'
import Link from 'next/link'

export default function Feature({
  number,
  title,
  subtitle,
  infoSections,
  cta,
  image,
}) {
  return (
    <div className={s.featureContainer}>
      <div className={s.featureText}>
        <div className={s.listNumber}>
          <span className="g-type-display-5">{number}</span>
        </div>
        <div>
          <h3 className="g-type-display-2">{title}</h3>
          <p className="g-type-body-large ">{subtitle}</p>
          {infoSections.map(({ heading, content }) => (
            <div className={s.infoSection} key={`${title}-${heading}`}>
              <h4 className="g-type-display-5">{title}</h4>
              {content}
            </div>
          ))}
        </div>
        <Link href={cta.url}>
          <a className={s.cta}>
            <span>{cta.text}</span>
            <img src="/img/icons/arrow.svg" alt="cta-arrow" />
          </a>
        </Link>
      </div>
      <div className={s.featureImage}>
        <img src={image} alt={title} />
      </div>
    </div>
  )
}
