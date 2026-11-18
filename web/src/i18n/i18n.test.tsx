import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import dayjs from 'dayjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, errorMessage } from '../lib/api';
import { formatDuration, formatRange, formatSlot, formatSlotTime } from '../lib/slots';
import { Dashboard } from '../pages/Dashboard';
import { fakeApi, renderWithApp, summaryFixture } from '../test/utils';
import i18n, { currentLanguage, LANGUAGE_STORAGE_KEY } from '.';
import { en } from './en';
import { fr } from './fr';

function browserLanguages(langs: string[]) {
  vi.spyOn(navigator, 'languages', 'get').mockReturnValue(langs);
  vi.spyOn(navigator, 'language', 'get').mockReturnValue(langs[0]);
}

afterEach(() => vi.restoreAllMocks());

describe('language detection', () => {
  it('follows the browser language', async () => {
    browserLanguages(['fr-FR', 'en-US']);
    await i18n.changeLanguage();
    expect(currentLanguage()).toBe('fr');
    expect(document.documentElement.lang).toBe('fr');
    expect(dayjs.locale()).toBe('fr');
  });

  it('falls back to English for unsupported languages', async () => {
    browserLanguages(['de-DE']);
    await i18n.changeLanguage();
    expect(currentLanguage()).toBe('en');
  });

  it('prefers an explicit choice', async () => {
    browserLanguages(['en-US']);
    localStorage.setItem(LANGUAGE_STORAGE_KEY, 'fr');
    await i18n.changeLanguage();
    expect(currentLanguage()).toBe('fr');
  });

  it('defaults to English when nothing is resolved', () => {
    vi.spyOn(i18n, 'resolvedLanguage', 'get').mockReturnValue(undefined);
    expect(currentLanguage()).toBe('en');
  });
});

describe('translations', () => {
  const keys = (o: object, prefix = ''): string[] =>
    Object.entries(o).flatMap(([k, v]: [string, unknown]) =>
      v && typeof v === 'object' && !Array.isArray(v) ? keys(v, `${prefix}${k}.`) : [`${prefix}${k}`],
    );

  it('has the same keys in every language', () => {
    expect(keys(fr).sort()).toEqual(keys(en).sort());
  });

  it('translates API errors by code', async () => {
    await i18n.changeLanguage('fr');
    expect(errorMessage(new ApiError(409, 'this poll is closed', 'poll_closed'))).toBe('Ce sondage est clos.');
    expect(errorMessage(new ApiError(500, 'raw message', 'unknown_code'))).toBe('raw message');
    expect(errorMessage(new ApiError(500, 'raw message'))).toBe('raw message');
    expect(errorMessage('weird')).toBe('Une erreur est survenue');
  });

  it('formats dates and durations in French', async () => {
    await i18n.changeLanguage('fr');
    const day = { start: '2026-12-01T00:00:00Z', end: '2026-12-02T00:00:00Z' };
    expect(formatSlot(day, 'days')).toBe('mar. 1 déc. 2026');
    expect(formatSlotTime(day, 'days')).toBe('Toute la journée');
    expect(formatRange(day.start, '2026-12-05T00:00:00Z', 'days')).toBe('1 déc. – 4 déc. 2026');
    expect(formatDuration(150)).toBe('2 h 30');
    expect(formatDuration(60)).toBe('1 h');
  });
});

describe('LanguagePicker', () => {
  it('switches the interface language and remembers it', async () => {
    const api = fakeApi({ listEvents: vi.fn().mockResolvedValue([summaryFixture({ responseCount: 0 })]) });
    renderWithApp(<Dashboard />, { api });
    expect(await screen.findByText('Welcome back, Ada')).toBeInTheDocument();
    // The picker lives in the layout; render it standalone here.
    const { LanguagePicker } = await import('../components/LanguagePicker');
    renderWithApp(<LanguagePicker />);
    await userEvent.click(await screen.findByRole('button', { name: 'Language' }));
    await userEvent.click(await screen.findByText('Français'));
    expect(await screen.findByText('Bon retour, Ada')).toBeInTheDocument();
    // French uses the singular for zero.
    expect(screen.getByText('0 réponse')).toBeInTheDocument();
    expect(screen.getByText('3 journées proposées')).toBeInTheDocument();
    expect(localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe('fr');
    expect(screen.getByRole('button', { name: 'Langue' })).toHaveTextContent('fr');
  });
});
