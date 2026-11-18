import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { alan, fakeApi, renderWithApp, summaryFixture } from '../test/utils';
import { Dashboard } from './Dashboard';

describe('Dashboard', () => {
  it('shows a loading state', async () => {
    renderWithApp(<Dashboard />, { api: fakeApi({ listEvents: vi.fn(() => new Promise(() => {})) as never }) });
    expect(await screen.findByTestId('loading')).toBeInTheDocument();
  });

  it('shows the empty state', async () => {
    renderWithApp(<Dashboard />);
    expect(await screen.findByText('No events yet')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Create an event/ })).toHaveAttribute('href', '/new');
  });

  it('shows errors', async () => {
    renderWithApp(<Dashboard />, { api: fakeApi({ listEvents: vi.fn().mockRejectedValue(new Error('boom')) }) });
    expect(await screen.findByText('boom')).toBeInTheDocument();
  });

  it('lists, filters and searches events', async () => {
    const events = [
      summaryFixture(),
      summaryFixture({
        id: 'ev2',
        title: 'Sprint review',
        location: 'Zoom',
        role: 'participant',
        owner: alan,
        closed: true,
        mode: 'hours',
        slotCount: 1,
        responseCount: 1,
        firstStart: undefined,
        lastEnd: undefined,
      }),
    ];
    renderWithApp(<Dashboard />, { api: fakeApi({ listEvents: vi.fn().mockResolvedValue(events) }) });
    const card = await screen.findByRole('link', { name: 'Team offsite' });
    expect(card).toHaveAttribute('href', '/e/ev1');
    expect(screen.getByText('Dec 1 – Dec 3, 2026')).toBeInTheDocument();
    expect(screen.getByText('3 day options')).toBeInTheDocument();
    expect(screen.getByText('1 time option')).toBeInTheDocument();
    expect(screen.getByText('No dates')).toBeInTheDocument();
    expect(screen.getByText('Closed')).toBeInTheDocument();

    await userEvent.click(screen.getByText('Participating'));
    expect(screen.queryByText('Team offsite')).not.toBeInTheDocument();
    expect(screen.getByText('Sprint review')).toBeInTheDocument();

    await userEvent.click(screen.getByText('Organizing'));
    expect(screen.getByText('Team offsite')).toBeInTheDocument();

    await userEvent.click(screen.getByText('All (2)'));
    await userEvent.type(screen.getByLabelText('Search events'), 'zoom');
    expect(screen.getByText('Sprint review')).toBeInTheDocument();
    expect(screen.queryByText('Team offsite')).not.toBeInTheDocument();

    await userEvent.type(screen.getByLabelText('Search events'), 'xyz');
    expect(screen.getByText('No events match your filters.')).toBeInTheDocument();
  });

  it('pluralizes counts', async () => {
    const e = summaryFixture({ slotCount: 1, responseCount: 2, mode: 'days' });
    renderWithApp(<Dashboard />, { api: fakeApi({ listEvents: vi.fn().mockResolvedValue([e]) }) });
    expect(await screen.findByText('1 day option')).toBeInTheDocument();
    expect(screen.getByText('2 responses')).toBeInTheDocument();
  });
});
