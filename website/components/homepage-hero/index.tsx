import TextSplit from '@hashicorp/react-text-split'
import VideoCarousel from '@hashicorp/react-hero/carousel'
import s from './style.module.css'

export default function HomepageHero({ title, description, links, videos }) {
  return (
    <div className={s.homepageHero}>
      <TextSplit
        product="consul"
        heading={title}
        content={description}
        links={links}
        linkStyle="buttons"
      >
        <VideoCarousel videos={videos} />
      </TextSplit>
    </div>
  )
}
