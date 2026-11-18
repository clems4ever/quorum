import { render } from '@testing-library/react';
import type { ReactElement, ReactNode } from 'react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { vi, type Mock } from 'vitest';
import { Providers } from '../App';
import type { Api } from '../lib/api';
import { useAuth } from '../lib/auth-context';
import type { EventDetail, EventSummary, User } from '../lib/types';

export const ada: User = { id: 'u1', email: 'ada@example.com', name: 'Ada Lovelace', picture: '' };
export const alan: User = { id: 'u2', email: 'alan@example.com', name: 'Alan Turing', picture: 'https://x/a.png' };

export function fakeApi(overrides: Partial<Api> = {}): { [K in keyof Api]: Mock<Api[K]> } {
  return {
    me: vi.fn().mockResolvedValue(ada),
    logout: vi.fn().mockResolvedValue(undefined),
    listEvents: vi.fn().mockResolvedValue([]),
    getEvent: vi.fn().mockResolvedValue(eventFixture()),
    createEvent: vi.fn().mockResolvedValue(eventFixture()),
    updateEvent: vi.fn().mockResolvedValue(eventFixture()),
    deleteEvent: vi.fn().mockResolvedValue(undefined),
    closeEvent: vi.fn().mockResolvedValue(eventFixture({ closed: true })),
    reopenEvent: vi.fn().mockResolvedValue(eventFixture()),
    respond: vi.fn().mockResolvedValue(eventFixture()),
    withdraw: vi.fn().mockResolvedValue(eventFixture()),
    uploadImage: vi.fn().mockResolvedValue(eventFixture()),
    deleteImage: vi.fn().mockResolvedValue(eventFixture()),
    setImageFocus: vi.fn().mockResolvedValue(eventFixture()),
    onUnauthorized: vi.fn(() => () => {}),
    ...overrides,
  } as never;
}

export function eventFixture(over: Partial<EventDetail> = {}): EventDetail {
  return {
    id: 'ev1',
    title: 'Team offsite',
    description: 'Planning days',
    location: 'Lyon',
    mode: 'days',
    owner: ada,
    slots: [
      { id: 's1', start: '2026-12-01T00:00:00Z', end: '2026-12-02T00:00:00Z' },
      { id: 's2', start: '2026-12-02T00:00:00Z', end: '2026-12-03T00:00:00Z' },
      { id: 's3', start: '2026-12-03T00:00:00Z', end: '2026-12-04T00:00:00Z' },
    ],
    closed: false,
    createdAt: '2026-11-01T00:00:00Z',
    updatedAt: '2026-11-01T00:00:00Z',
    responses: [{ user: alan, votes: { s1: 'yes', s2: 'maybe' }, updatedAt: '2026-11-02T00:00:00Z' }],
    ...over,
  };
}

export function summaryFixture(over: Partial<EventSummary> = {}): EventSummary {
  return {
    id: 'ev1',
    title: 'Team offsite',
    location: 'Lyon',
    mode: 'days',
    owner: ada,
    role: 'owner',
    closed: false,
    slotCount: 3,
    responseCount: 1,
    firstStart: '2026-12-01T00:00:00Z',
    lastEnd: '2026-12-04T00:00:00Z',
    createdAt: '2026-11-01T00:00:00Z',
    updatedAt: '2026-11-01T00:00:00Z',
    ...over,
  };
}

function WhenSignedIn({ children }: { children: ReactNode }) {
  return useAuth().user ? <>{children}</> : null;
}

/** Renders ui inside the app providers and a memory router. */
export function renderWithApp(
  ui: ReactElement,
  {
    api = fakeApi(),
    route = '/',
    path = '*',
  }: { api?: ReturnType<typeof fakeApi>; route?: string; path?: string } = {},
) {
  const result = render(
    <Providers api={api}>
      <MemoryRouter initialEntries={[route]}>
        <Routes>
          <Route path={path} element={<WhenSignedIn>{ui}</WhenSignedIn>} />
          <Route path="*" element={<div data-testid="elsewhere" />} />
        </Routes>
      </MemoryRouter>
    </Providers>,
  );
  return { ...result, api };
}
