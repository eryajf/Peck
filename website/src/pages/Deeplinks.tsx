import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import SEO from '../components/SEO'
import { config } from '../config'
import { deeplinks } from '../data/deeplinks'
import { LanguageSwitch, useLanguage } from '../i18n'
import './deeplinks.css'

function CopyButton({ value }: { value: string }) {
  const { t } = useLanguage()
  const [status, setStatus] = useState<'idle' | 'copied' | 'error'>('idle')
  async function handleCopy() {
    try { await navigator.clipboard.writeText(value); setStatus('copied') }
    catch { setStatus('error') }
    window.setTimeout(() => setStatus('idle'), 2000)
  }
  return <button type="button" className="directory-button directory-button-secondary" onClick={handleCopy} aria-live="polite">{status === 'copied' ? t('已复制', 'Copied') : status === 'error' ? t('复制失败，请手动复制', 'Copy failed; select the link') : t('复制链接', 'Copy link')}</button>
}

export default function Deeplinks({ embedded = false }: { embedded?: boolean }) {
  const { t, language } = useLanguage()
  const [query, setQuery] = useState('')
  const [activeCategory, setActiveCategory] = useState('all')
  const categories = useMemo(() => Array.from(new Set(deeplinks.map((item) => item.category))), [])
  const filteredLinks = useMemo(() => {
    const term = query.trim().toLowerCase()
    return deeplinks.filter((item) => (activeCategory === 'all' || item.category === activeCategory) && [item.name, item.description, item.descriptionEn, item.id, item.status, item.category, item.deeplink].join(' ').toLowerCase().includes(term))
  }, [activeCategory, query])

  return <div className={`directory ${embedded ? 'directory-embedded' : ''}`}>
    {!embedded && <SEO title={`Deeplinks — ${config.name}`} description={t('浏览并一键打开 Peck 准备链接。', 'Explore and open Peck preparation links.')} url={config.siteUrl ? `${config.siteUrl}/deeplinks` : ''} />}
    <header className="directory-header"><Link to="/" className="directory-brand" aria-label={t('返回 Peck 首页', 'Back to Peck home')}><img src="/images/peck-mark-color.svg" alt="" /><strong>Peck</strong><span>/</span><span className="directory-brand-page">Deeplinks</span></Link><nav aria-label={t('页面导航', 'Page navigation')}><Link className="directory-home" to="/">{t('返回首页', 'Home')}</Link><LanguageSwitch /><a className="directory-repo" href={config.social.github} target="_blank" rel="noopener noreferrer">GitHub <span>↗</span></a></nav></header>
    <main>
      <section className="directory-hero"><div className="directory-hero-inner"><div className="directory-hero-copy"><p className="directory-eyebrow"><span /> PECK DIRECTORY</p><h1>{t('从这里，', 'A little shortcut')}<br /><em>{t('开始准备。', 'to get started.')}</em></h1><p>{t('通过 Deeplink 唤起 Peck 的准备界面。检查与处理应用的每一步，仍由你来决定。', 'Open Peck’s preparation screen with a Deeplink. You remain in control of every step of inspecting and processing an app.')}</p><div className="directory-hero-actions"><a href="#directory-title" className="directory-hero-link">{t('浏览入口', 'Explore links')} <span>↓</span></a><div className="directory-hero-note"><span>✳</span>{t('使用前请先安装 Peck', 'Install Peck before opening a link')}</div></div></div><div className="directory-illustration" aria-hidden="true"><div className="directory-orbit orbit-a" /><div className="directory-orbit orbit-b" /><div className="directory-illustration-card"><div className="directory-icon"><img src="/images/peck-mark-color.svg" alt="" /></div><span>peck://prepare</span><strong>↗</strong></div><span className="directory-spark spark-a">✳</span><span className="directory-spark spark-b">✦</span></div></div></section>
      <section className="directory-content" aria-labelledby="directory-title"><div className="directory-heading"><div><span className="directory-kicker">EXPLORE / 01</span><h2 id="directory-title">{t('找到你的入口。', 'Find your starting point.')}</h2><p>{filteredLinks.length} {t('个入口', filteredLinks.length === 1 ? 'link' : 'links')}</p></div><label className="directory-search"><span aria-hidden="true">⌕</span><span className="sr-only">{t('搜索 Deeplinks', 'Search Deeplinks')}</span><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('搜索项目、描述或协议…', 'Search projects, descriptions or protocols…')} /></label></div>
        <div className="directory-filters" aria-label={t('分类筛选', 'Filter by category')}><button type="button" aria-pressed={activeCategory === 'all'} onClick={() => setActiveCategory('all')}>{t('全部', 'All')}</button>{categories.map((category) => <button key={category} type="button" aria-pressed={activeCategory === category} onClick={() => setActiveCategory(category)}>{category}</button>)}</div>
        <div className="directory-grid">{filteredLinks.map((item) => <article className="directory-card" key={item.id}><div className="directory-card-top"><div className="directory-card-identity"><div className="directory-card-icon">{<img src={item.icon} alt="" loading="lazy" referrerPolicy="no-referrer" onError={(event) => { event.currentTarget.onerror = null; event.currentTarget.src = '/images/peck-mark-color.svg' }} />}</div><div><h3>{item.name}</h3><p>{item.category}</p></div></div><span className="directory-platform">macOS</span></div><p className="directory-description">{language === 'en' ? item.descriptionEn ?? item.description : item.description}</p><div className="directory-status"><span>{t('项目状态', 'Project status')}</span><p lang="zh-CN">{item.status}</p>{language === 'en' && <small>Project status is currently available in Chinese.</small>}<a href={item.statusUrl} target="_blank" rel="noopener noreferrer">{t('查看来源', 'View source')} ↗</a></div><div className="directory-protocol"><span>DEEPLINK</span><code>{item.deeplink}</code></div><div className="directory-card-actions"><a href={item.deeplink} className="directory-button directory-button-primary">{t('一键打开', 'Open in Peck')} <span>↗</span></a><CopyButton value={item.deeplink} />{item.repository && <a href={item.repository} target="_blank" rel="noopener noreferrer" className="directory-project">{t('查看项目', 'View project')} ↗</a>}</div></article>)}</div>
        {filteredLinks.length === 0 && <div className="directory-empty"><span>⌕</span><p>{t('没有找到匹配的入口。', 'No matching links found.')}</p><small>{t('试试其他关键词或分类。', 'Try another search or category.')}</small></div>}
      </section>
      <section className="directory-contribute"><div><span className="directory-kicker">BUILD WITH PECK / 02</span><h2>{t('让你的项目，也能从这里出发。', 'Bring Peck to your project.')}</h2><p>{t('了解如何从项目页面唤起 Peck 的准备流程。', 'Learn how to open Peck’s preparation flow from your project page.')}</p></div><a href={config.social.github} target="_blank" rel="noopener noreferrer">{t('查看项目文档', 'Read the docs')} <span>↗</span></a></section>
    </main><footer className="directory-footer"><Link to="/" className="directory-brand"><img src="/images/peck-mark-color.svg" alt="" /><strong>Peck</strong></Link><p>{t('为 Mac 上的每一次打开，啄掉一点障碍。', 'Peck away at the obstacles to opening apps on your Mac.')}</p><a href={config.social.github}>GitHub ↗</a></footer>
  </div>
}
