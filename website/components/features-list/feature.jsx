import Button from '@hashicorp/react-button'
import s from './style.module.css'

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
      <img src={image} alt={title} />
      <div className={s.featureTextContainer}>
        <span className={s.listNumber}>{number}</span>
        <div className={s.featureText}>
          <h3 className={s.featureTitle}>{title}</h3>
          <p className={s.featureSubtitle}>{subtitle}</p>
          <div className={s.infoSection}>
            {infoSections.map(({ heading, content }) => (
              <div className={s.infoSectionItem} key={`${title}-${heading}`}>
                <h4 className={s.infoTitle}>{heading}</h4>
                {content}
              </div>
            ))}
          </div>
          <div className={s.ctaContainer}>
            <Button
              title={cta.text}
              url={cta.url}
              linkType="inbound"
              theme={{
                brand: 'neutral',
                variant: 'primary',
              }}
            />
          </div>
        </div>
      </div>
    </div>
  )
}
