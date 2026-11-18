import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ApiError } from '../lib/api';
import { ada, alan, eventFixture, fakeApi, renderWithApp } from '../test/utils';
import { EventPage } from './EventPage';

const render = (api = fakeApi()) => renderWithApp(<EventPage />, { api, route: '/e/ev1', path: '/e/:id' });

const myCell = (n: number) => screen.getAllByRole('button', { name: /^Your vote for/ })[n];

describe('EventPage', () => {
  it('shows event details, votes and the best slot', async () => {
    render();
    expect(await screen.findByRole('heading', { name: 'Team offsite' })).toBeInTheDocument();
    expect(screen.getByText('Organized by you')).toBeInTheDocument();
    expect(screen.getByText('Lyon')).toBeInTheDocument();
    expect(screen.getByText('Planning days')).toBeInTheDocument();
    expect(screen.getByText('Open for votes')).toBeInTheDocument();
    expect(screen.getByText('Alan Turing')).toBeInTheDocument();
    expect(screen.getByTestId('best-s1')).toBeInTheDocument();
    expect(screen.getByText('Tue, Dec 1, 2026')).toBeInTheDocument();
    expect(screen.getByTestId('yes-s1')).toHaveTextContent('1');
  });

  it('reports missing events and other errors', async () => {
    render(fakeApi({ getEvent: vi.fn().mockRejectedValue(new ApiError(404, 'event not found')) }));
    expect(await screen.findByText('This event does not exist or was deleted.')).toBeInTheDocument();
  });

  it('reports generic load errors', async () => {
    render(fakeApi({ getEvent: vi.fn().mockRejectedValue(new Error('offline')) }));
    expect(await screen.findByText('offline')).toBeInTheDocument();
  });

  it('lets a participant vote', async () => {
    const api = fakeApi({
      getEvent: vi.fn().mockResolvedValue(eventFixture({ owner: alan, responses: [] })),
      respond: vi
        .fn()
        .mockResolvedValue(
          eventFixture({ owner: alan, responses: [{ user: ada, votes: { s1: 'yes' }, updatedAt: '' }] }),
        ),
    });
    render(api);
    expect(await screen.findByText('Organized by Alan Turing')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Event actions' })).not.toBeInTheDocument();
    expect(screen.queryByText(/Most popular so far/)).not.toBeInTheDocument();

    await userEvent.click(myCell(0)); // yes
    await userEvent.click(myCell(1)); // yes
    await userEvent.click(myCell(1)); // maybe
    await userEvent.click(myCell(2));
    await userEvent.click(myCell(2));
    await userEvent.click(myCell(2)); // back to no
    expect(myCell(1)).toHaveAccessibleName(/If need be/);
    await userEvent.click(screen.getByRole('button', { name: 'Save my votes' }));
    expect(api.respond).toHaveBeenCalledWith('ev1', { s1: 'yes', s2: 'maybe', s3: 'no' });
    expect(await screen.findByText('Your votes were saved')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Update my votes' })).toBeDisabled();
  });

  it('supports yes to all, reset and withdraw', async () => {
    const event = eventFixture({ responses: [{ user: ada, votes: { s2: 'yes' }, updatedAt: '' }] });
    const api = fakeApi({ getEvent: vi.fn().mockResolvedValue(event) });
    render(api);
    await screen.findByRole('heading', { name: 'Team offsite' });
    await userEvent.click(screen.getByRole('button', { name: 'Yes to all' }));
    expect(screen.getByRole('button', { name: 'Update my votes' })).toBeEnabled();
    await userEvent.click(screen.getByRole('button', { name: 'Reset' }));
    expect(screen.getByRole('button', { name: 'Update my votes' })).toBeDisabled();
    await userEvent.click(screen.getByRole('button', { name: 'Withdraw my response' }));
    expect(api.withdraw).toHaveBeenCalledWith('ev1');
  });

  it('notifies about failures', async () => {
    render(fakeApi({ respond: vi.fn().mockRejectedValue(new Error('this poll is closed')) }));
    await screen.findByRole('heading', { name: 'Team offsite' });
    await userEvent.click(myCell(0));
    await userEvent.click(screen.getByRole('button', { name: 'Save my votes' }));
    expect(await screen.findByText('this poll is closed')).toBeInTheDocument();
  });

  it('closes the poll with a final date', async () => {
    const api = fakeApi({
      closeEvent: vi.fn().mockResolvedValue(eventFixture({ closed: true, finalSlotId: 's2' })),
    });
    render(api);
    await userEvent.click(await screen.findByRole('button', { name: 'Event actions' }));
    await userEvent.click(await screen.findByText('Close poll'));
    const dialog = await screen.findByRole('dialog');
    // Most popular option is preselected.
    expect(within(dialog).getAllByRole('radio')[0]).toBeChecked();
    expect(within(dialog).getByText('1 maybe')).toBeInTheDocument();
    await userEvent.click(within(dialog).getByText('Wed, Dec 2, 2026'));
    await userEvent.click(within(dialog).getByRole('button', { name: 'Close poll' }));
    expect(api.closeEvent).toHaveBeenCalledWith('ev1', 's2');
    expect(await screen.findByText('The date is set!')).toBeInTheDocument();
    expect(screen.getByText('Chosen')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Your vote for/ })).not.toBeInTheDocument();
  });

  it('closes without a date, keeps the modal open on failure and cancels', async () => {
    const api = fakeApi({ closeEvent: vi.fn().mockRejectedValueOnce(new Error('nope')) });
    render(api);
    await userEvent.click(await screen.findByRole('button', { name: 'Event actions' }));
    await userEvent.click(await screen.findByText('Close poll'));
    const dialog = await screen.findByRole('dialog');
    await userEvent.click(within(dialog).getByText('Close without choosing a date'));
    await userEvent.click(within(dialog).getByRole('button', { name: 'Close poll' }));
    expect(api.closeEvent).toHaveBeenCalledWith('ev1', undefined);
    expect(await screen.findByText('nope')).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('shows closed polls read-only and reopens them', async () => {
    const closed = eventFixture({
      closed: true,
      responses: [
        { user: ada, votes: { s3: 'yes' }, updatedAt: '' },
        { user: alan, votes: {}, updatedAt: '' },
      ],
    });
    const api = fakeApi({ getEvent: vi.fn().mockResolvedValue(closed) });
    render(api);
    expect(await screen.findByText(/This poll is closed/)).toBeInTheDocument();
    expect(screen.getByText('Ada Lovelace (you)')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Event actions' }));
    await userEvent.click(await screen.findByText('Reopen poll'));
    expect(api.reopenEvent).toHaveBeenCalledWith('ev1');
    expect(await screen.findByText('The poll is open again')).toBeInTheDocument();
  });

  it('deletes the event after confirmation', async () => {
    const api = fakeApi({
      deleteEvent: vi.fn().mockRejectedValueOnce(new Error('denied')).mockResolvedValue(undefined),
    });
    render(api);
    await userEvent.click(await screen.findByRole('button', { name: 'Event actions' }));
    await userEvent.click(await screen.findByText('Delete event'));
    const dialog = await screen.findByRole('dialog');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await userEvent.click(screen.getByRole('button', { name: 'Event actions' }));
    await userEvent.click(await screen.findByText('Delete event'));
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Delete' }));
    expect(await screen.findByText('denied')).toBeInTheDocument();
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Delete' }));
    expect(await screen.findByTestId('elsewhere')).toBeInTheDocument();
  });

  it('copies the share link', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    render();
    await userEvent.click(await screen.findByRole('button', { name: 'Share' }));
    expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/e/ev1`);
    expect(await screen.findByRole('button', { name: 'Link copied' })).toBeInTheDocument();
  });

  it('shows hour slots with their time range', async () => {
    const event = eventFixture({
      mode: 'hours',
      slots: [{ id: 's1', start: '2026-12-01T09:00:00Z', end: '2026-12-01T10:00:00Z' }],
      responses: [],
    });
    render(fakeApi({ getEvent: vi.fn().mockResolvedValue(event) }));
    expect(await screen.findByText('09:00 – 10:00')).toBeInTheDocument();
    expect(screen.getByText('Time slots')).toBeInTheDocument();
  });
});
