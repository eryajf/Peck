import { useEffect, useMemo, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import SEO from '../components/SEO'
import { config } from '../config'
import { deeplinks, type DeeplinkEntry } from '../data/deeplinks'
import { LanguageSwitch, useLanguage } from '../i18n'
import './deeplinks.css'

const iconURL = 'https://peck.eryajf.net/images/peck-icon-animated.svg'

function CopyButton({ value, label }: { value: string; label: string }) {
  const { t } = useLanguage()
  const [status, setStatus] = useState<'idle' | 'copied' | 'error'>('idle')

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(value)
      setStatus('copied')
    } catch {
      setStatus('error')
    }
    window.setTimeout(() => setStatus('idle'), 2200)
  }

  return <button type="button" className="directory-copy" onClick={handleCopy} aria-live="polite">{status === 'copied' ? t('已复制', 'Copied') : status === 'error' ? t('复制失败', 'Copy failed') : label}</button>
}

function readmeSnippet(item: DeeplinkEntry, size: number, label: string) {
  const landing = `https://peck.eryajf.net/deeplinks?project=${encodeURIComponent(item.id)}#readme-title`
  return `<a href="${landing}">
  <img src="${iconURL}" width="${size}" height="${size}" alt=""> ${label}
</a>`
}

export default function Deeplinks({ embedded = false }: { embedded?: boolean }) {
  const { t, language } = useLanguage()
  const { hash, search } = useLocation()
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState('all')
  const [iconSize, setIconSize] = useState(24)
  const [selection, setSelection] = useState<{ search: string; id: string } | null>(null)
  const requested = new URLSearchParams(search).get('project')
  const selectedProject = selection?.search === search ? selection.id : requested === null ? deeplinks[0]?.id ?? '' : deeplinks.find((item) => item.id === requested)?.id ?? ''
  useEffect(() => {
    if (embedded || hash !== '#readme-title') return
    const frame = requestAnimationFrame(() => document.getElementById('readme-title')?.scrollIntoView())
    return () => cancelAnimationFrame(frame)
  }, [embedded, hash])
  const term = query.trim().toLowerCase()
  const categories = useMemo(() => Array.from(new Map(deeplinks.map((item) => [item.category, item.categoryEn])).entries()), [])
  const filteredLinks = useMemo(() => deeplinks.filter((item) => (category === 'all' || item.category === category) && [item.name, item.description, item.descriptionEn, item.category, item.categoryEn, item.bundleId, item.deeplink].join(' ').toLowerCase().includes(term)), [category, term])
  const selectedEntry = deeplinks.find((item) => item.id === selectedProject)
  const snippet = selectedEntry ? readmeSnippet(selectedEntry, iconSize, t('交给 Peck 处理', 'Hand it to Peck')) : ''

  return <div className={`directory ${embedded ? 'directory-embedded' : ''}`}>
    {!embedded && <SEO title={`Deeplinks — ${config.name}`} description={t('打开需要处理的 Mac 应用，或把 Peck 放进你的 README。', 'Open Mac apps that need attention, or bring Peck to your README.')} url={config.siteUrl ? `${config.siteUrl}/deeplinks` : ''} />}
    <header className="directory-header">
      <Link to="/" className="directory-brand" aria-label={t('返回 Peck 首页', 'Back to Peck home')}><img src="/images/peck-icon-animated.svg" alt="" /><strong>Peck</strong><span>/</span><span className="directory-brand-page">Deeplinks</span></Link>
      <nav aria-label={t('页面导航', 'Page navigation')}><Link className="directory-home" to="/">{t('返回首页', 'Home')}</Link><LanguageSwitch /><a className="directory-repo" href={config.social.github} target="_blank" rel="noopener noreferrer">GitHub <span>↗</span></a></nav>
    </header>

    <main>
      <section className="directory-hero">
        <div className="directory-hero-inner">
          <div className="directory-hero-copy">
            <p className="directory-eyebrow"><span /> PECK / OPEN SOURCE LINKS</p>
            <h1>{t('把打不开的应用，', 'Give stuck apps')}<br /><em>{t('交给 Peck。', 'a way forward.')}</em></h1>
            <p>{t('这里收录正在处理或需要核验的开源 Mac 应用。点击入口，Peck 会在本机找到对应应用，并把每一步交还给你确认。', 'A small directory for open source Mac apps that need checking. Open an entry and Peck will find the local app, then leave every decision to you.')}</p>
            <div className="directory-hero-actions"><a href="#directory-title" className="directory-hero-link">{t('查看可用入口', 'Browse available links')} <span>↓</span></a><span className="directory-hero-note"><b>{String(deeplinks.length).padStart(2, '0')}</b> {t('个待处理项目', 'projects to check')}</span></div>
          </div>
          <div className="directory-illustration" aria-hidden="true">
            <div className="directory-illustration-grid" />
            <div className="directory-orbit orbit-a" /><div className="directory-orbit orbit-b" />
            <div className="directory-illustration-card"><span className="directory-icon"><img src="/images/peck-mark-color.svg" alt="" /></span><span><small>READY TO OPEN</small><strong>peck://prepare</strong></span><b>↗</b></div>
            <span className="directory-spark spark-a">✳</span><span className="directory-spark spark-b">✦</span>
          </div>
        </div>
      </section>

      <section className="directory-content" aria-labelledby="directory-title">
        <div className="directory-heading"><div><span className="directory-kicker">DIRECTORY / 01</span><h2 id="directory-title">{t('正在等待处理。', 'Ready when you are.')}</h2><p>{filteredLinks.length} {t('个入口 · 均来自开源项目', 'links · all from open source projects')}</p></div></div>
        <div className="directory-toolbar"><div className="directory-filters" role="group" aria-label={t('按分类筛选', 'Filter by category')}><button type="button" aria-pressed={category === 'all'} onClick={() => setCategory('all')}>{t('全部', 'All')}</button>{categories.map(([name, nameEn]) => <button type="button" key={name} aria-pressed={category === name} onClick={() => setCategory(name)}>{language === 'en' ? nameEn : name}</button>)}</div><label className="directory-search"><span aria-hidden="true">⌕</span><span className="sr-only">{t('搜索 Deeplinks', 'Search Deeplinks')}</span><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('搜索项目或 Bundle ID…', 'Search projects or bundle ID…')} /></label></div>
        <div className="directory-grid">{filteredLinks.map((item, index) => <article className="directory-card" key={item.id}>
          <div className="directory-card-index">{String(index + 1).padStart(2, '0')} / {language === 'en' ? item.categoryEn : item.category}</div>
          <div className="directory-card-top"><div className="directory-card-identity"><div className="directory-card-icon"><img src={item.icon} alt="" loading="lazy" onError={(event) => { event.currentTarget.onerror = null; event.currentTarget.src = '/images/peck-mark-color.svg' }} /></div><div><h3>{item.name}</h3><p>{language === 'en' ? item.descriptionEn : item.description}</p></div></div></div>
          <div className="directory-card-meta"><span>Bundle ID</span><code>{item.bundleId}</code></div>
          <div className="directory-card-actions"><a href={item.deeplink} className="directory-button directory-button-primary">{t('一键打开 Peck', 'Open in Peck')} <span>↗</span></a><CopyButton value={item.deeplink} label={t('复制 Deeplink', 'Copy Deeplink')} /><a href={item.repository} target="_blank" rel="noopener noreferrer" className="directory-project">{t('项目仓库', 'Repository')} ↗</a></div>
        </article>)}</div>
        {filteredLinks.length === 0 && <div className="directory-empty"><span>⌕</span><p>{t('没有找到匹配的项目。', 'No matching project found.')}</p><small>{t('试试其他关键词或分类。', 'Try another search or category.')}</small></div>}
      </section>

      <section className="directory-readme" aria-labelledby="readme-title">
        <div className="directory-readme-copy"><span className="directory-kicker">SHARE THE SHORTCUT / 02</span><h2 id="readme-title">{t('把 Peck 放进你的 README。', 'Put Peck in your README.')}</h2><p>{t('已收录项目可以把下面的图标放在 README 里。用户点击后进入项目的 Peck 页面，再点击图标唤起 Peck；HTTPS 中转方便兼容 GitHub 等平台。尚未收录的项目可以先提交到目录。', 'Listed projects can put this icon in their README. It links to the project on Peck’s HTTPS page, where users click again to open Peck. Projects not yet listed can submit an entry first.')}</p><ol className="directory-steps"><li><b>01</b><span>{t('从目录中选择已收录的项目。', 'Choose a project already in the directory.')}</span></li><li><b>02</b><span>{t('把图标链接放进 README。', 'Add the button image to the README.')}</span></li><li><b>03</b><span>{t('用户在 Peck 页面点击图标，并在应用中确认处理。', 'Users click the badge on Peck’s page and confirm in the app.')}</span></li></ol></div>
        <div className="directory-readme-demo"><div className="directory-demo-label"><span>LIVE EXAMPLE</span><span>README / LINK</span></div><div className="directory-demo-stage">{selectedEntry ? <><a href={selectedEntry.deeplink} className="directory-peck-button"><img src="/images/peck-icon-animated.svg" width={iconSize} height={iconSize} style={{ width: iconSize, height: iconSize }} alt="" /><span>{t('交给 Peck 处理', 'Hand it to Peck')}</span></a><p>{t('请先安装并运行过 Peck，再点击图标打开对应应用的准备界面。', 'Install and run Peck first, then click the icon to open this app’s preparation screen.')}</p></> : <p role="status">{t('未找到链接中的项目，请从下方选择项目或提交新项目。', 'The linked project was not found. Choose a project below or submit a new one.')}</p>}</div><label className="directory-demo-select">{t('选择示例项目', 'Choose an example')}<select value={selectedProject} onChange={(event) => setSelection({ search, id: event.target.value })}>{!selectedEntry && <option value="" disabled>{t('请选择项目', 'Choose a project')}</option>}{deeplinks.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</select></label><label className="directory-demo-select directory-size-select">{t('图标尺寸', 'Icon size')}<select value={iconSize} onChange={(event) => setIconSize(Number(event.target.value))}><option value={20}>20 × 20 px</option><option value={24}>24 × 24 px {t('（推荐）', '(recommended)')}</option><option value={32}>32 × 32 px</option></select></label><p className="directory-size-note">{t('也可以在 README 代码中调整 width 和 height。', 'You can also adjust width and height in the README code.')}</p>{selectedEntry && <div className="directory-code-wrap"><code>{snippet}</code><CopyButton value={snippet} label={t('复制 README 代码', 'Copy README code')} /></div>}</div>
      </section>

      <section className="directory-contribute"><div><span className="directory-kicker">BUILD WITH PECK / 03</span><h2>{t('你的项目，也可以成为下一个入口。', 'Your project can be the next entry.')}</h2><p>{t('准备好 Bundle ID、项目主页和一条说明，提交后即可把处理入口交给更多用户。', 'Bring a Bundle ID, a project homepage, and a short note. Then give more users a clear path to the fix.')}</p></div><a href={`${config.social.github}/blob/main/data/project-links.json`} target="_blank" rel="noopener noreferrer">{t('查看数据格式', 'View data format')} <span>↗</span></a></section>
    </main>
    <footer className="directory-footer"><Link to="/" className="directory-brand"><img src="/images/peck-icon-animated.svg" alt="" /><strong>Peck</strong></Link><p>{t('为 Mac 上的每一次打开，啄掉一点障碍。', 'Peck away at the obstacles to opening apps on your Mac.')}</p><a href={config.social.github} target="_blank" rel="noopener noreferrer">GitHub ↗</a></footer>
  </div>
}
