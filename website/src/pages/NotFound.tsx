import { Link } from 'react-router-dom'
import SEO from '../components/SEO'
import { config } from '../config'
import { LanguageSwitch, useLanguage } from '../i18n'

export default function NotFound() {
  const { t } = useLanguage()
  return (
    <div className="terminal-theme">
      <SEO
        title={`404 — ${t("页面不存在", "Page Not Found")} | ${config.username}`}
        description={t("你访问的页面不存在。", "The page you are looking for does not exist.")}
      />
      <div className="terminal-container flex flex-col items-center justify-center min-h-[80vh] text-center">
        <LanguageSwitch />
        <h1 className="text-6xl font-bold text-[var(--accent-color)] mb-4">404</h1>
        <p className="text-[var(--text-secondary)] mb-2 text-lg">{t('页面不存在', 'Page not found')}</p>
        <p className="text-[var(--text-secondary)] text-sm mb-8">bash: page: command not found</p>
        <Link to="/" className="inline-block text-[var(--accent-color)] border border-[var(--accent-color)] px-6 py-2 text-sm hover:bg-[var(--accent-color)] hover:text-[var(--bg-color)] transition-colors !border-b-[var(--accent-color)]">
          cd ~/ &rarr;
        </Link>
      </div>
    </div>
  )
}
