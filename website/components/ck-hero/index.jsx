import Button from '@hashicorp/react-button'
import s from './style.module.css'

export default function CKHero({ title, subtitle, ctas, media }) {
  return (
    <div
      className={s.ckHero}
      style={{
        backgroundImage: `url(${require('./img/background-design.svg')})`,
      }}
    >
      <div className="g-grid-container">
        <div className={s.headline}>
          <h1 className="g-type-display-1">{title}</h1>
          <p className="g-type-body-large">{subtitle}</p>
          {ctas.map(({ text, url }, idx) => (
            <Button
              key={text}
              theme={{
                brand: idx === 0 ? 'consul' : 'neutral',
                variant: 'primary',
              }}
              linkType={idx === 0 ? null : 'inbound'}
              url={url}
              title={text}
              className={idx === 0 ? null : s.inboundButton}
            />
          ))}
        </div>
        {media && media.type === 'image' ? (
          <div className="image">
            <img alt={media.alt} src={media.source} />
          </div>
        ) : media && media.type === 'video' ? (
          <div className="video">
            <video muted playsInline>
              <source src={media.source} type="video/mp4" />
            </video>
          </div>
        ) : null}
      </div>
    </div>
  )
}
