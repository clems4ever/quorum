import dayjs from 'dayjs';
import 'dayjs/locale/fr';
import i18n from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';
import { en } from './en';
import { fr } from './fr';

export const languages = [
  { code: 'en', label: 'English' },
  { code: 'fr', label: 'Français' },
] as const;

export type Language = (typeof languages)[number]['code'];

export const LANGUAGE_STORAGE_KEY = 'quorum.lang';

/** Keeps dayjs and the document language in sync with i18next. */
function applyLanguage() {
  // The raw language may be a browser tag such as "fr-FR"; use the supported one.
  const lng = currentLanguage();
  dayjs.locale(lng);
  document.documentElement.lang = lng;
}

i18n.on('languageChanged', applyLanguage);

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: { en: { translation: en }, fr: { translation: fr } },
    supportedLngs: languages.map((l) => l.code),
    nonExplicitSupportedLngs: true,
    load: 'languageOnly',
    fallbackLng: 'en',
    interpolation: { escapeValue: false },
    detection: {
      // An explicit choice wins, then the browser languages. Only explicit
      // choices are stored (see chooseLanguage) so the browser setting keeps
      // applying until the user picks a language.
      order: ['localStorage', 'navigator'],
      lookupLocalStorage: LANGUAGE_STORAGE_KEY,
      caches: [],
    },
  });

/** Switches to a language picked by the user and remembers the choice. */
export function chooseLanguage(lng: Language) {
  localStorage.setItem(LANGUAGE_STORAGE_KEY, lng);
  return i18n.changeLanguage(lng);
}

/** The active supported language ("en" or "fr"). */
export function currentLanguage(): Language {
  return (i18n.resolvedLanguage as Language | undefined) ?? 'en';
}

export default i18n;
