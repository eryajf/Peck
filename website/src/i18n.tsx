import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'

type Language = 'zh' | 'en'
interface LanguageContextValue { language: Language; setLanguage: (language: Language) => void; t: (zh: string, en: string) => string }
const LanguageContext = createContext<LanguageContextValue | null>(null)

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [language, setLanguage] = useState<Language>(() => {
    try {
      const saved = localStorage.getItem('peck-language')
      if (saved === 'zh' || saved === 'en') return saved
    } catch { /* Storage can be unavailable in private browsing. */ }
    return navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en'
  })
  useEffect(() => {
    document.documentElement.lang = language === 'zh' ? 'zh-CN' : 'en'
    try { localStorage.setItem('peck-language', language) } catch { /* The switch still works for this visit. */ }
  }, [language])
  return <LanguageContext.Provider value={{ language, setLanguage, t: (zh, en) => language === 'zh' ? zh : en }}>{children}</LanguageContext.Provider>
}

// The provider and hook live together to keep language state in one place.
// eslint-disable-next-line react-refresh/only-export-components
export function useLanguage() {
  const context = useContext(LanguageContext)
  if (!context) throw new Error('useLanguage must be used within LanguageProvider')
  return context
}

export function LanguageSwitch() {
  const { language, setLanguage } = useLanguage()
  return <div className="language-switch" role="group" aria-label={language === 'zh' ? '选择语言' : 'Choose language'}>
    <button type="button" lang="zh-CN" aria-pressed={language === 'zh'} onClick={() => setLanguage('zh')}>中文</button>
    <button type="button" lang="en" aria-pressed={language === 'en'} onClick={() => setLanguage('en')}>EN</button>
  </div>
}
