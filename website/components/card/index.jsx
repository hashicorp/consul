import InlineSvg from '@hashicorp/react-inline-svg'
import svgDownload from './icons/download.svg?include'
import svgVideo from './icons/video.svg?include'
import s from './card.module.css'

const iconDict = {
  download: svgDownload,
  video: svgVideo,
}

function Card({ label, icon, url, brand, themedCard, children }) {
  const useCardTheme = brand && themedCard
  return (
    <a
      href={url}
      className={s.card}
      style={{
        '--brand': `var(--${brand})`,
        '--brand-text': `var(--${brand}-text)`,
        '--background-color': useCardTheme
          ? `var(--${brand}-l3) `
          : 'var(--white)',
        '--border-color': useCardTheme
          ? `var(--${brand}-l2) `
          : 'var(--gray-6)',
      }}
    >
      <div>
        {icon != null ? (
          <InlineSvg className={s.icon} src={iconDict[icon]} />
        ) : null}
        {label != null ? <span className={s.label}>{label}</span> : null}
        <span className={s.cardContent}>{children}</span>
      </div>
    </a>
  )
}

export default Card
