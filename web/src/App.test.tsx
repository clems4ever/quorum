import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { App, AppRoutes, Providers } from './App';
import type { Api } from './lib/api';
import { useAuth } from './lib/auth-context';
import { ada, fakeApi, summaryFixture } from './test/utils';

function renderRoutes(api: Api, route: string) {
  return render(
    <Providers api={api}>
      <MemoryRouter initialEntries={[route]}>
        <AppRoutes />
      </MemoryRouter>
    </Providers>,
  );
}

describe('App', () => {
  it('shows the landing page when signed out', async () => {
    renderRoutes(fakeApi({ me: vi.fn().mockResolvedValue(null) }), '/');
    expect(await screen.findByText(/works for everyone/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Sign in with Google' })).toHaveAttribute('href', '/auth/login?next=%2F');
    expect(screen.queryByText(/Sign in to view this event/)).not.toBeInTheDocument();
  });

  it('keeps the requested page as sign-in target', async () => {
    renderRoutes(fakeApi({ me: vi.fn().mockResolvedValue(null) }), '/e/abc?x=1');
    expect(await screen.findByText(/Sign in to view this event/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Sign in with Google' })).toHaveAttribute(
      'href',
      '/auth/login?next=%2Fe%2Fabc%3Fx%3D1',
    );
  });

  it('treats a failing session check as signed out', async () => {
    renderRoutes(fakeApi({ me: vi.fn().mockRejectedValue(new Error('down')) }), '/');
    expect(await screen.findByText(/works for everyone/)).toBeInTheDocument();
  });

  it('shows the dashboard when signed in and redirects unknown routes', async () => {
    const api = fakeApi({ listEvents: vi.fn().mockResolvedValue([summaryFixture()]) });
    renderRoutes(api, '/does/not/exist');
    expect(await screen.findByText('Welcome back, Ada')).toBeInTheDocument();
    expect(await screen.findByText('Team offsite')).toBeInTheDocument();
  });

  it('signs out from the account menu', async () => {
    const api = fakeApi();
    renderRoutes(api, '/');
    await userEvent.click(await screen.findByRole('button', { name: 'Account menu' }));
    expect(await screen.findByText(ada.email)).toBeInTheDocument();
    await userEvent.click(screen.getByText('Sign out'));
    expect(api.logout).toHaveBeenCalled();
    expect(await screen.findByText(/works for everyone/)).toBeInTheDocument();
  });

  it('returns to the sign-in page with a notice when the session expires', async () => {
    let expire: () => void = () => {};
    const api = fakeApi({
      onUnauthorized: vi.fn((l: () => void) => {
        expire = l;
        return () => {};
      }),
    });
    renderRoutes(api, '/new');
    expect(await screen.findByText('Propose a new event')).toBeInTheDocument();
    act(() => expire());
    expect(await screen.findByText('Your session has expired')).toBeInTheDocument();
    expect(screen.queryByText(/Sign in to view this event/)).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Sign in with Google' })).toHaveAttribute(
      'href',
      '/auth/login?next=%2Fnew',
    );
  });

  it('does not report an expired session for anonymous visitors', async () => {
    let expire: () => void = () => {};
    const api = fakeApi({
      me: vi.fn().mockResolvedValue(null),
      onUnauthorized: vi.fn((l: () => void) => {
        expire = l;
        return () => {};
      }),
    });
    renderRoutes(api, '/');
    await screen.findByText(/works for everyone/);
    act(() => expire());
    expect(screen.queryByText('Your session has expired')).not.toBeInTheDocument();
  });

  it('renders with a browser router', async () => {
    render(<App api={fakeApi()} />);
    expect(await screen.findByText('Welcome back, Ada')).toBeInTheDocument();
  });

  it('ignores the session result after unmount', async () => {
    let resolve: (v: null) => void = () => {};
    const api = fakeApi({ me: vi.fn(() => new Promise((r) => (resolve = r))) as never });
    const { unmount } = renderRoutes(api, '/');
    unmount();
    await act(() => Promise.resolve().then(() => resolve(null)));
  });

  it('exposes a default auth context', () => {
    let state: ReturnType<typeof useAuth> | undefined;
    function Probe() {
      state = useAuth();
      return null;
    }
    render(<Probe />);
    expect(state?.loading).toBe(true);
    return state?.signOut();
  });

  it('toggles the color scheme', async () => {
    renderRoutes(fakeApi(), '/');
    const toggle = await screen.findByRole('button', { name: 'Switch to dark theme' });
    await userEvent.click(toggle);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Switch to light theme' })).toBeInTheDocument());
    await userEvent.click(screen.getByRole('button', { name: 'Switch to light theme' }));
    expect(await screen.findByRole('button', { name: 'Switch to dark theme' })).toBeInTheDocument();
  });
});
