import svgDownload from './icons/download.svg?include'
import svgVideo from './icons/video.svg?include'
import svgPlay from './icons/play.svg?include'
import svgClose from './icons/close.svg?include'
import svgCloseRefined from './icons/close-refined.svg?include'
import style from './icon.module.css'

const iconDict = {
  download: svgDownload,
  video: svgVideo,
  play: svgPlay,
  close: svgClose,
  closeRefined: svgCloseRefined,
}

function Icon({ icon }) {
  const svgString = iconDict[icon]
  return (
    <div
      className={style.icon}
      dangerouslySetInnerHTML={{
        __html: svgString,
      }}
    />
  )
}

export default Icon
