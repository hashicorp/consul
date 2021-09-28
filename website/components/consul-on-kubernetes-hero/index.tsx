import Button from '@hashicorp/react-button'
import s from './style.module.css'

interface Cta {
  url: string
  text: string
}

interface ConsulOnKubernetesHeroProps {
  title: string
  description: string
  ctas: Cta[]
  videoSource: string
}

export default function ConsulOnKubernetesHero({
  title,
  description,
  ctas,
  videoSource,
}: ConsulOnKubernetesHeroProps) {
  return (
    <div
      className={s.ckHero}
      style={{
        backgroundImage: `url(${require('./images/background-design.svg')})`,
      }}
    >
      <div className={s.contentWrapper}>
        <div className={s.headline}>
          <h1 className={s.title}>{title}</h1>
          <p className={s.description}>{description}</p>
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
          <iframe
            width="720"
            height="315"
            src="https://www.youtube.com/embed/Eyszp_apaEU"
            title="YouTube video player"
            frameBorder="0"
            allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
            allowFullScreen
          />
        </div>
      </div>
    </div>
  )
}
