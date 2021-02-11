import { useState } from 'react'
import InlineSvg from '@hashicorp/react-inline-svg'
import svgPlay from './play.svg?include'
import s from './introduction.module.css'
import Modal from '../modal'

// TODO - we could rename this component's folder and module.css file to video-modal,
// didn't want to jump the gun on that just to keep the diff clean.
function VideoModal({ brand, videoEmbedSrc, title, presenter, presenterRole }) {
  const [isModalShown, setIsModalShown] = useState(false)

  return (
    <div
      // Set the brand CSS variables used in the component based on the brand prop
      // Note: we have an RFC related to theming that might be worth referencing
      // once it has been finalized:
      // https://app.asana.com/0/1100423001970639/1199652658988771/f
      style={{
        '--brand': `var(--${brand})`,
        '--brand-l1': `var(--${brand}-l1)`,
      }}
    >
      <Modal show={isModalShown} close={() => setIsModalShown(false)}>
        <iframe
          title={title}
          className={s.videoIframe}
          src={videoEmbedSrc}
          frameBorder="0"
          allowFullScreen
        ></iframe>
      </Modal>
      <div className={s.video}>
        <button className={s.button} onClick={() => setIsModalShown(true)}>
          <InlineSvg src={svgPlay} />
        </button>
        <div className={s.content}>
          <span className={s.title}>{title}</span>
          <span className={s.presenter}>{presenter}</span>
          <span className={s.presenterRole}>{presenterRole}</span>
        </div>
      </div>
    </div>
  )
}

export default VideoModal
