import InlineSvg from '@hashicorp/react-inline-svg'
import svgCloseRefined from './close-refined.svg?include'
import s from './modal.module.css'

function Modal({ show, close, children }) {
  if (!show) return null
  return (
    <div className={s.container}>
      <div className={s.scrim} onClick={() => close(false)}></div>
      <div className={s.contentsContainer} onClick={() => close(false)}>
        <div className={s.grid}>
          <div className={s.contents}>{children}</div>
          <button className={s.close} onClick={() => close(false)}>
            <InlineSvg src={svgCloseRefined} />
          </button>
        </div>
      </div>
    </div>
  )
}

export default Modal
