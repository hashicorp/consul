import s from './style.module.css'
import Hero from '@hashicorp/react-hero'

export default function HomepageHero({
  title,
  description,
  links,
  uiVideo,
  cliVideo,
  alert,
  image,
}) {
  return (
    <div className={s.consulHero}>
      <Hero
        data={{
          product: 'consul',
          alert: alert ? { ...alert, tagColor: 'consul-pink' } : null,
          title: title,
          description: description,
          buttons: links,
          backgroundTheme: 'light',
          centered: false,
          image: image ? { ...image } : null,
          videos: [
            ...(uiVideo
              ? [
                  {
                    name: uiVideo.name ?? 'UI',
                    playbackRate: uiVideo.playbackRate,
                    aspectRatio: uiVideo.aspectRatio,
                    src: [
                      {
                        srcType: uiVideo.srcType,
                        url: uiVideo.url,
                      },
                    ],
                  },
                ]
              : []),
            ...(cliVideo
              ? [
                  {
                    name: cliVideo.name ?? 'CLI',
                    playbackRate: cliVideo.playbackRate,
                    aspectRatio: cliVideo.aspectRatio,
                    src: [
                      {
                        srcType: cliVideo.srcType,
                        url: cliVideo.url,
                      },
                    ],
                  },
                ]
              : []),
          ],
        }}
      />
    </div>
  )
}
