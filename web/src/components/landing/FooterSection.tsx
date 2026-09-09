import { t, Language } from '../../i18n/translations'

interface FooterSectionProps {
  language: Language
}

export default function FooterSection({ language }: FooterSectionProps) {
  const links = {
    supporters: [
      { name: 'Binance', href: 'https://www.maxweb.red/join?ref=AUAIEXAI' },
      {
        name: 'Amber.ac',
        href: 'https://amber.ac/',
        badge: language === 'zh' ? '战略投资' : 'Strategic',
      },
    ],
  }

  return (
    <footer style={{ background: '#0B0E11', borderTop: '1px solid rgba(255, 255, 255, 0.06)' }}>
      <div className="max-w-6xl mx-auto px-4 py-12">
        {/* Top Section */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-10 mb-12">
          {/* Brand */}
          <div className="md:col-span-2">
            <div className="flex items-center gap-3 mb-4">
              <img src="/icons/auaiex.svg" alt="AUAIEX Logo" className="w-8 h-8" />
              <span className="text-xl font-bold" style={{ color: '#EAECEF' }}>
                AUAIEX
              </span>
            </div>
            <p className="text-sm mb-6" style={{ color: '#5E6673' }}>
              {t('futureStandardAI', language)}
            </p>
          </div>

          {/* Supporters */}
          <div>
            <h4 className="text-sm font-semibold mb-4" style={{ color: '#EAECEF' }}>
              {t('supporters', language)}
            </h4>
            <ul className="space-y-3">
              {links.supporters.map((link) => (
                <li key={link.name}>
                  <a
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm transition-colors hover:text-[#F0B90B] inline-flex items-center gap-2"
                    style={{ color: '#5E6673' }}
                  >
                    {link.name}
                    {link.badge && (
                      <span
                        className="text-xs px-1.5 py-0.5 rounded"
                        style={{
                          background: 'rgba(240, 185, 11, 0.1)',
                          color: '#F0B90B',
                        }}
                      >
                        {link.badge}
                      </span>
                    )}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        </div>

        {/* Bottom Section */}
        <div
          className="pt-6 text-center text-xs"
          style={{ color: '#5E6673', borderTop: '1px solid rgba(255, 255, 255, 0.06)' }}
        >
          <p className="mb-2">{t('footerTitle', language)}</p>
          <p style={{ color: '#3C4249' }}>{t('footerWarning', language)}</p>
        </div>
      </div>
    </footer>
  )
}
