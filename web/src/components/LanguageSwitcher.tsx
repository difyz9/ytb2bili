'use client';

import { Languages } from 'lucide-react';
import { useI18n } from '@/contexts/I18nContext';
import { SUPPORTED_LOCALES, getLocaleLabel } from '@/lib/i18n';

export function LanguageSwitcher() {
  const { locale, setLocale, t } = useI18n();

  return (
    <div
      className="inline-flex items-center gap-1 rounded-full border border-slate-200 bg-white/90 p-1 text-sm text-slate-700 shadow-sm backdrop-blur"
      role="group"
      aria-label={t('Language')}
    >
      <Languages className="h-4 w-4" />
      <span className="sr-only">{t('Language')}</span>
      {SUPPORTED_LOCALES.map((item) => {
        const active = item === locale;

        return (
          <button
            key={item}
            type="button"
            onClick={() => setLocale(item)}
            aria-pressed={active}
            className={`rounded-full px-3 py-1 text-sm transition-colors ${active ? 'bg-slate-900 text-white' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'}`}
          >
            {getLocaleLabel(item)}
          </button>
        );
      })}
    </div>
  );
}
