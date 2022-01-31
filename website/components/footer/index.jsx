import Link from 'next/link'

export default function Footer({ openConsentManager }) {
  return (
    <footer className="g-footer">
      <div className="g-grid-container">
        <div className="left">
          <Link href="/intro">
            <a>Intro</a>
          </Link>
          <Link href="/docs/guides">
            <a>Guides</a>
          </Link>
          <Link href="/docs">
            <a>Docs</a>
          </Link>
          <Link href="/community">
            <a>Community</a>
          </Link>
          <a href="https://hashicorp.com/privacy">Privacy</a>
          <Link href="/security">
            <a>Security</a>
          </Link>
          <a href="https://www.hashicorp.com/brand">Press Kit</a>
          <a onClick={openConsentManager}>Consent Manager</a>
        </div>
      </div>
    </footer>
  )
}
