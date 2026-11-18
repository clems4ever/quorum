import { describe, expect, it, vi } from 'vitest';
import { ApiError, createApi, errorMessage } from './api';

function respond(status: number, body?: unknown, json = true) {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    statusText: 'Status ' + status,
    json: json ? () => Promise.resolve(body) : () => Promise.reject(new Error('no json')),
  });
}

describe('createApi', () => {
  it('sends requests with JSON bodies', async () => {
    const f = respond(200, { id: 'e1' });
    const api = createApi(f);
    const input = { title: 't', description: '', location: '', mode: 'days' as const, slots: [] };
    await expect(api.createEvent(input)).resolves.toEqual({ id: 'e1' });
    expect(f).toHaveBeenCalledWith('/api/events', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    });
  });

  it('maps every endpoint', async () => {
    const f = respond(200, {});
    const api = createApi(f);
    const input = { title: 't', description: '', location: '', mode: 'days' as const, slots: [] };
    await api.listEvents();
    await api.getEvent('a/b');
    await api.updateEvent('e', input);
    await api.closeEvent('e', 's1');
    await api.closeEvent('e');
    await api.reopenEvent('e');
    await api.respond('e', { s1: 'yes' });
    await api.withdraw('e');
    const calls = (f.mock.calls as [string, RequestInit][]).map(
      ([url, init]) => `${init.method} ${url} ${typeof init.body === 'string' ? init.body : ''}`,
    );
    expect(calls).toEqual([
      'GET /api/events ',
      'GET /api/events/a%2Fb ',
      `PUT /api/events/e ${JSON.stringify(input)}`,
      'POST /api/events/e/close {"finalSlotId":"s1"}',
      'POST /api/events/e/close {}',
      'POST /api/events/e/reopen ',
      'PUT /api/events/e/response {"votes":{"s1":"yes"}}',
      'DELETE /api/events/e/response ',
    ]);
  });

  it('handles empty responses', async () => {
    const f = respond(204);
    const api = createApi(f);
    await expect(api.deleteEvent('e')).resolves.toBeUndefined();
    await expect(api.logout()).resolves.toBeUndefined();
    expect(f).toHaveBeenLastCalledWith('/auth/logout', expect.objectContaining({ method: 'POST' }));
  });

  it('turns errors into ApiError', async () => {
    const api = createApi(respond(400, { error: 'title is required' }));
    await expect(api.listEvents()).rejects.toEqual(new ApiError(400, 'title is required'));
    const api2 = createApi(respond(502, undefined, false));
    await expect(api2.listEvents()).rejects.toThrow('Status 502');
  });

  it('returns null from me() when signed out', async () => {
    await expect(createApi(respond(401, { error: 'x' })).me()).resolves.toBeNull();
    await expect(createApi(respond(200, { id: 'u' })).me()).resolves.toEqual({ id: 'u' });
    await expect(createApi(respond(500, { error: 'boom' })).me()).rejects.toThrow('boom');
  });

  it('notifies listeners when the session is rejected', async () => {
    const api = createApi(respond(401, { error: 'not signed in' }));
    const listener = vi.fn();
    const unsubscribe = api.onUnauthorized(listener);
    await expect(api.listEvents()).rejects.toThrow('not signed in');
    expect(listener).toHaveBeenCalledTimes(1);
    unsubscribe();
    await expect(api.listEvents()).rejects.toThrow();
    expect(listener).toHaveBeenCalledTimes(1);
  });

  it('uses the global fetch by default', async () => {
    const f = respond(200, []);
    vi.stubGlobal('fetch', f);
    await createApi().listEvents();
    expect(f).toHaveBeenCalled();
    vi.unstubAllGlobals();
  });
});

describe('errorMessage', () => {
  it('extracts messages', () => {
    expect(errorMessage(new Error('x'))).toBe('x');
    expect(errorMessage('nope')).toBe('Something went wrong');
  });
});
