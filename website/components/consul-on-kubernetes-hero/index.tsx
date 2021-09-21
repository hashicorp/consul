import Button from '@hashicorp/react-button'
import s from './style.module.css'

interface Cta {
  url: string
  text: string
}

interface Media {
  type: 'image' | 'video'
  source: string
  alt?: string
}

interface ConsulOnKubernetesHeroProps {
  title: string
  subtitle: string
  ctas: Cta[]
  media: Media
}

export default function ConsulOnKubernetesHero({
  title,
  subtitle,
  ctas,
  media,
}: ConsulOnKubernetesHeroProps) {
  return (
    <div
      className={s.ckHero}
      style={{
        backgroundImage: `url(${require('./img/background-design.svg')})`,
      }}
    >
      <div className={s.contentWrapper}>
        <div className={s.headline}>
          <h1 className={s.title}>{title}</h1>
          <p className={s.subtitle}>{subtitle}</p>
          <div className={s.buttons}>
            {ctas.map(({ text, url }, idx) => (
              <Button
                key={text}
                theme={{
                  brand: 'consul',
                  variant: idx === 0 ? 'primary' : 'tertiary-neutral',
                  background: 'dark',
                }}
                linkType={idx === 0 ? null : 'inbound'}
                url={url}
                title={text}
                className={s.button}
              />
            ))}
          </div>
        </div>
        <div className={s.media}>
          {media.type === 'image' ? (
            <img alt={media.alt} src={media.source} />
          ) : (
            <video>
              <source src={media.source} type="video/mp4" />
            </video>
          )}
        </div>
      </div>
    </div>
  )
}
