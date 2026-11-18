import { createContext, useContext } from 'react';
import i18n from '../i18n';
import type { EventDetail, EventInput, EventSummary, Focus, User, VoteOrNo } from './types';

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    /** Stable error code sent by the server, used for translations. */
    public code?: string,
  ) {
    super(message);
  }
}

export interface Api {
  me(): Promise<User | null>;
  logout(): Promise<void>;
  listEvents(): Promise<EventSummary[]>;
  getEvent(id: string): Promise<EventDetail>;
  createEvent(input: EventInput): Promise<EventDetail>;
  updateEvent(id: string, input: EventInput): Promise<EventDetail>;
  deleteEvent(id: string): Promise<void>;
  closeEvent(id: string, finalSlotId?: string): Promise<EventDetail>;
  reopenEvent(id: string): Promise<EventDetail>;
  respond(id: string, votes: Record<string, VoteOrNo>): Promise<EventDetail>;
  withdraw(id: string): Promise<EventDetail>;
  uploadImage(id: string, image: Blob, focus: Focus): Promise<EventDetail>;
  setImageFocus(id: string, focus: Focus): Promise<EventDetail>;
  deleteImage(id: string): Promise<EventDetail>;
  /** Registers a listener called whenever the server reports that the session is gone. */
  onUnauthorized(listener: () => void): () => void;
}

type Fetch = typeof fetch;

/** The JSON body of a failed API request. */
interface ErrorBody {
  error?: string;
  code?: string;
}

function isErrorBody(v: unknown): v is ErrorBody {
  return typeof v === 'object' && v !== null;
}

/** Creates an API client backed by the given fetch implementation. */
export function createApi(fetchFn: Fetch = (...args) => fetch(...args)): Api {
  const listeners = new Set<() => void>();

  /** Sends a request; a Blob body is sent as is, anything else as JSON. */
  async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
    const blob = body instanceof Blob;
    const res = await fetchFn(path, {
      method,
      credentials: 'same-origin',
      headers:
        body === undefined
          ? undefined
          : { 'Content-Type': blob ? body.type || 'application/octet-stream' : 'application/json' },
      body: body === undefined ? undefined : blob ? body : JSON.stringify(body),
    });
    if (res.status === 204) {
      return undefined as T;
    }
    const data: unknown = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (res.status === 401) listeners.forEach((l) => l());
      const body = isErrorBody(data) ? data : {};
      throw new ApiError(res.status, body.error ?? (res.statusText || 'Request failed'), body.code);
    }
    return data as T;
  }

  const ev = (id: string) => `/api/events/${encodeURIComponent(id)}`;

  return {
    async me() {
      try {
        return await call<User>('GET', '/api/me');
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) return null;
        throw e;
      }
    },
    logout: () => call('POST', '/auth/logout'),
    listEvents: () => call('GET', '/api/events'),
    getEvent: (id) => call('GET', ev(id)),
    createEvent: (input) => call('POST', '/api/events', input),
    updateEvent: (id, input) => call('PUT', ev(id), input),
    deleteEvent: (id) => call('DELETE', ev(id)),
    closeEvent: (id, finalSlotId) => call('POST', `${ev(id)}/close`, finalSlotId ? { finalSlotId } : {}),
    reopenEvent: (id) => call('POST', `${ev(id)}/reopen`),
    respond: (id, votes) => call('PUT', `${ev(id)}/response`, { votes }),
    withdraw: (id) => call('DELETE', `${ev(id)}/response`),
    uploadImage: (id, image, focus) => call('PUT', `${ev(id)}/image?x=${focus.x}&y=${focus.y}`, image),
    setImageFocus: (id, focus) => call('PATCH', `${ev(id)}/image`, { focus }),
    deleteImage: (id) => call('DELETE', `${ev(id)}/image`),
    onUnauthorized(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}

export const ApiContext = createContext<Api | null>(null);

export function useApi(): Api {
  const api = useContext(ApiContext);
  if (!api) throw new Error('useApi must be used inside an ApiContext provider');
  return api;
}

/** A user facing, translated message for an error. */
export function errorMessage(e: unknown): string {
  if (e instanceof ApiError && e.code && i18n.exists(`apiErrors.${e.code}`)) {
    return i18n.t(`apiErrors.${e.code}`);
  }
  return e instanceof Error ? e.message : i18n.t('common.somethingWrong');
}
