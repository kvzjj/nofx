import { createContext, useContext, useState, ReactNode } from 'react'
import type { Language } from '../i18n/translations'

interface LanguageContextType {
  language: Language
  setLanguage: (lang: Language) => void
}

const LanguageContext = createContext<LanguageContextType | undefined>(
  undefined
)

export function LanguageProvider({ children }: { children: ReactNode }) {
  // 初始化语言：优先用户已保存的选择；否则自动检测浏览器语言（中文环境默认中文，其余默认英文）
  const [language, setLanguage] = useState<Language>(() => {
    const saved = localStorage.getItem('language')
    if (saved === 'en' || saved === 'zh') return saved
    const browserLang =
      typeof navigator !== 'undefined'
        ? (navigator.language || '').toLowerCase()
        : ''
    return browserLang.startsWith('zh') ? 'zh' : 'en'
  })

  // Save language to localStorage whenever it changes
  const handleSetLanguage = (lang: Language) => {
    setLanguage(lang)
    localStorage.setItem('language', lang)
  }

  return (
    <LanguageContext.Provider
      value={{ language, setLanguage: handleSetLanguage }}
    >
      {children}
    </LanguageContext.Provider>
  )
}

export function useLanguage() {
  const context = useContext(LanguageContext)
  if (!context) {
    throw new Error('useLanguage must be used within LanguageProvider')
  }
  return context
}
