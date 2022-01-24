import Button from '@hashicorp/react-button'
import ReactPlayer from 'react-player'
import s from './style.module.css'

interface Cta {
  url: string
  text: string
}

interface ConsulOnKubernetesHeroProps {
  title: string
  description: string
  ctas: Cta[]
  video: {
    src: string
    poster: string
  }
}

export default function ConsulOnKubernetesHero({
  title,
  description,
  ctas,
  video,
}: ConsulOnKubernetesHeroProps) {
  return (
    <div className={s.ckHero}>
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
          <img
            src={require('./images/bg-top.svg')}
            alt="background top"
            className={s.bgTop}
          />
          <img
            src={require('./images/bg-right.svg')}
            alt="background right"
            className={s.bgRight}
          />
          <img
            src={require('./images/bg-dots.svg')}
            alt="background bottom"
            className={s.bgBottom}
          />
          <img
            src={require('./images/bg-dots.svg')}
            alt="background left"
            className={s.bgLeft}
          />
          <div className={s.video}>
            <ReactPlayer
              playing
              light={video.poster}
              url={video.src}
              width="100%"
              height="100%"
              controls
              className={s.player}
              playIcon={
                <svg
                  aria-label="Play video"
                  width="72"
                  height="72"
                  viewBox="0 0 72 72"
                  fill="none"
                  xmlns="http://www.w3.org/2000/svg"
                >
                  <rect width="72" height="72" rx="36" fill="#F85C94" />
                  <path d="M56 36L26 53.3205L26 18.6795L56 36Z" fill="white" />
                </svg>
              }
            />
          </div>
        </div>
      </div>
    </div>
  )
}
