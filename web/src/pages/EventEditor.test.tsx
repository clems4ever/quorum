import { fireEvent, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../lib/api';
import { alan, eventFixture, fakeApi, renderWithApp } from '../test/utils';
import { EventEditor } from './EventEditor';

const today = new Date('2026-11-18T12:00:00Z');

afterEach(() => sessionStorage.clear());

describe('EventEditor (create)', () => {
  it('validates required fields', async () => {
    const { api } = renderWithApp(<EventEditor today={today} />, { route: '/new', path: '/new' });
    await userEvent.click(await screen.findByRole('button', { name: 'Create event' }));
    expect(screen.getByText('Give your event a title')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('Select at least one option');
    expect(api.createEvent).not.toHaveBeenCalled();
  });

  it('creates a whole-day event', async () => {
    const { api } = renderWithApp(<EventEditor today={today} />, { route: '/new', path: '/new' });
    await userEvent.type(await screen.findByLabelText(/Title/), 'Offsite');
    await userEvent.type(screen.getByLabelText('Location'), 'Lyon');
    await userEvent.type(screen.getByLabelText('Description'), 'Fun');
    await userEvent.click(screen.getByRole('button', { name: '25 November 2026' }));
    await userEvent.click(screen.getByRole('button', { name: 'Create event' }));
    expect(api.createEvent).toHaveBeenCalledWith({
      title: 'Offsite',
      description: 'Fun',
      location: 'Lyon',
      mode: 'days',
      slots: [{ start: '2026-11-25T00:00:00.000Z', end: '2026-11-26T00:00:00.000Z' }],
    });
    expect(await screen.findByTestId('elsewhere')).toBeInTheDocument();
    expect(await screen.findByText('Event created')).toBeInTheDocument();
  });

  it('creates a time slot event and reports failures', async () => {
    const api = fakeApi({ createEvent: vi.fn().mockRejectedValue(new Error('title is too long')) });
    renderWithApp(<EventEditor today={today} />, { api, route: '/new', path: '/new' });
    await userEvent.type(await screen.findByLabelText(/Title/), 'Review');
    await userEvent.click(screen.getByText('Time slots'));
    expect(screen.getByText('Pick the time slots that could work.')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole('gridcell', { name: 'Thu Nov 19 09:00' }));
    fireEvent.mouseUp(window);
    await userEvent.click(screen.getByRole('button', { name: 'Create event' }));
    expect(api.createEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        mode: 'hours',
        slots: [{ start: '2026-11-19T09:00:00.000Z', end: '2026-11-19T10:00:00.000Z' }],
      }),
    );
    expect(await screen.findByText('title is too long')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create event' })).toBeEnabled();
  });
});

describe('EventEditor (edit)', () => {
  const render = (api = fakeApi()) =>
    renderWithApp(<EventEditor today={today} />, { api, route: '/e/ev1/edit', path: '/e/:id/edit' });

  it('loads and saves an existing event, keeping slot ids', async () => {
    const { api } = render();
    expect(await screen.findByDisplayValue('Team offsite')).toBeInTheDocument();
    expect(screen.getByText('Edit event')).toBeInTheDocument();
    const title = screen.getByLabelText(/Title/);
    await userEvent.clear(title);
    await userEvent.type(title, 'Renamed');
    await userEvent.click(screen.getByRole('button', { name: 'Remove 2026-12-02' }));
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(api.updateEvent).toHaveBeenCalled());
    const [id, input] = api.updateEvent.mock.calls[0];
    expect(id).toBe('ev1');
    expect(input.title).toBe('Renamed');
    expect(input.slots.map((s) => s.id)).toEqual(['s1', 's3']);
    expect(await screen.findByText('Event updated')).toBeInTheDocument();
  });

  it('refuses to edit events of other people', async () => {
    render(fakeApi({ getEvent: vi.fn().mockResolvedValue(eventFixture({ owner: alan })) }));
    expect(await screen.findByText('Only the organizer can edit this event.')).toBeInTheDocument();
  });

  it('shows load errors', async () => {
    render(fakeApi({ getEvent: vi.fn().mockRejectedValue(new Error('event not found')) }));
    expect(await screen.findByText('event not found')).toBeInTheDocument();
  });

  it('loads hour based events in the hour picker', async () => {
    const event = eventFixture({
      mode: 'hours',
      slots: [{ id: 'h1', start: '2026-12-01T09:00:00Z', end: '2026-12-01T10:00:00Z' }],
    });
    render(fakeApi({ getEvent: vi.fn().mockResolvedValue(event) }));
    expect(await screen.findByTestId('week-label')).toHaveTextContent('Nov 30 – Dec 6, 2026');
    expect(screen.getByTestId('slot-block')).toHaveTextContent('09:00–10:00');
  });
});

describe('EventEditor drafts', () => {
  const draft = {
    title: 'Draft title',
    description: 'd',
    location: 'l',
    mode: 'days',
    slots: [{ start: '2026-12-10T00:00:00.000Z', end: '2026-12-11T00:00:00.000Z' }],
  };

  it('saves new event drafts and clears them once created', async () => {
    const { api } = renderWithApp(<EventEditor today={today} />, { route: '/new', path: '/new' });
    await userEvent.type(await screen.findByLabelText(/Title/), 'Raclette');
    expect((JSON.parse(sessionStorage.getItem('quorum:draft:new')!) as { title: string }).title).toBe('Raclette');
    await userEvent.click(screen.getByRole('button', { name: '25 November 2026' }));
    await userEvent.click(screen.getByRole('button', { name: 'Create event' }));
    await waitFor(() => expect(api.createEvent).toHaveBeenCalled());
    await screen.findByTestId('elsewhere');
    expect(sessionStorage.getItem('quorum:draft:new')).toBeNull();
  });

  it('restores and discards a new event draft', async () => {
    sessionStorage.setItem('quorum:draft:new', JSON.stringify(draft));
    renderWithApp(<EventEditor today={today} />, { route: '/new', path: '/new' });
    expect(await screen.findByText('Unsaved draft restored')).toBeInTheDocument();
    expect(screen.getByDisplayValue('Draft title')).toBeInTheDocument();
    expect(screen.getByText('1 day selected')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Discard draft' }));
    expect(screen.queryByText('Unsaved draft restored')).not.toBeInTheDocument();
    expect(screen.getByLabelText(/Title/)).toHaveValue('');
    expect(sessionStorage.getItem('quorum:draft:new')).toBeNull();
  });

  it('ignores corrupted drafts', async () => {
    sessionStorage.setItem('quorum:draft:new', '{');
    renderWithApp(<EventEditor today={today} />, { route: '/new', path: '/new' });
    expect(await screen.findByLabelText(/Title/)).toHaveValue('');
    expect(screen.queryByText('Unsaved draft restored')).not.toBeInTheDocument();
  });

  it('restores edit drafts over the loaded event and reloads on discard', async () => {
    sessionStorage.setItem('quorum:draft:ev1', JSON.stringify(draft));
    const { api } = renderWithApp(<EventEditor today={today} />, { route: '/e/ev1/edit', path: '/e/:id/edit' });
    expect(await screen.findByDisplayValue('Draft title')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Discard draft' }));
    expect(await screen.findByDisplayValue('Team offsite')).toBeInTheDocument();
    expect(api.getEvent).toHaveBeenCalledTimes(2);
  });

  it('drops the draft on cancel', async () => {
    renderWithApp(<EventEditor today={today} />, { route: '/new', path: '/new' });
    await userEvent.type(await screen.findByLabelText(/Title/), 'x');
    await userEvent.click(screen.getByRole('link', { name: 'Cancel' }));
    expect(sessionStorage.getItem('quorum:draft:new')).toBeNull();
  });
});

describe('EventEditor cover image', () => {
  const photo = new File(['x'], 'photo.jpg', { type: 'image/jpeg' });
  const chooseImage = async (container: HTMLElement) => {
    fireEvent.change(container.querySelector('input[type=file]')!, { target: { files: [photo] } });
    expect(await screen.findByRole('img', { name: 'Cover image' })).toBeInTheDocument();
  };

  it('uploads the image after creating the event', async () => {
    const { api, container } = renderWithApp(<EventEditor today={today} />, { route: '/new', path: '/new' });
    await userEvent.type(await screen.findByLabelText(/Title/), 'Raclette');
    await userEvent.click(screen.getByRole('button', { name: '25 November 2026' }));
    await chooseImage(container);
    await userEvent.click(screen.getByRole('button', { name: 'Create event' }));
    await waitFor(() => expect(api.uploadImage).toHaveBeenCalledWith('ev1', photo, { x: 0.5, y: 0.5 }));
    expect(await screen.findByTestId('elsewhere')).toBeInTheDocument();
    expect(api.deleteImage).not.toHaveBeenCalled();
  });

  it('keeps the saved event when the upload fails', async () => {
    const api = fakeApi({ uploadImage: vi.fn().mockRejectedValue(new ApiError(413, 'too large', 'image_too_large')) });
    const { container } = renderWithApp(<EventEditor today={today} />, { api, route: '/new', path: '/new' });
    await userEvent.type(await screen.findByLabelText(/Title/), 'Raclette');
    await userEvent.click(screen.getByRole('button', { name: '25 November 2026' }));
    await chooseImage(container);
    await userEvent.click(screen.getByRole('button', { name: 'Create event' }));
    expect(await screen.findByText('The event was saved, but its image could not be updated')).toBeInTheDocument();
    expect(screen.getByText('The image is too large (10 MB maximum).')).toBeInTheDocument();
    expect(await screen.findByTestId('elsewhere')).toBeInTheDocument();
  });

  it('shows, replaces and removes the image of an existing event', async () => {
    const api = fakeApi({
      getEvent: vi
        .fn()
        .mockResolvedValue(eventFixture({ cover: { url: '/api/events/ev1/image?v=1', focus: { x: 0.2, y: 0.8 } } })),
    });
    renderWithApp(<EventEditor today={today} />, {
      api,
      route: '/e/ev1/edit',
      path: '/e/:id/edit',
    });
    expect(await screen.findByRole('img', { name: 'Cover image' })).toHaveAttribute('src', '/api/events/ev1/image?v=1');
    await userEvent.click(screen.getByRole('button', { name: 'Remove' }));
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(api.deleteImage).toHaveBeenCalledWith('ev1'));
    expect(api.uploadImage).not.toHaveBeenCalled();
    await screen.findByTestId('elsewhere');
  });

  it('replaces the image of an existing event', async () => {
    const api = fakeApi({
      getEvent: vi
        .fn()
        .mockResolvedValue(eventFixture({ cover: { url: '/api/events/ev1/image?v=1', focus: { x: 0.2, y: 0.8 } } })),
    });
    const { container } = renderWithApp(<EventEditor today={today} />, {
      api,
      route: '/e/ev1/edit',
      path: '/e/:id/edit',
    });
    await screen.findByRole('img', { name: 'Cover image' });
    fireEvent.change(container.querySelector('input[type=file]')!, { target: { files: [photo] } });
    await waitFor(() =>
      expect(screen.getByRole('img', { name: 'Cover image' })).toHaveAttribute('src', 'blob:photo.jpg'),
    );
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(api.uploadImage).toHaveBeenCalledWith('ev1', photo, { x: 0.5, y: 0.5 }));
    expect(api.deleteImage).not.toHaveBeenCalled();
  });

  it('saves only the new position of an existing image', async () => {
    const api = fakeApi({
      getEvent: vi.fn().mockResolvedValue(eventFixture({ cover: { url: '/img?v=1', focus: { x: 0.5, y: 0.5 } } })),
    });
    renderWithApp(<EventEditor today={today} />, { api, route: '/e/ev1/edit', path: '/e/:id/edit' });
    (await screen.findByTestId('cover-frame')).focus();
    await userEvent.keyboard('{ArrowDown}');
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await waitFor(() => expect(api.setImageFocus).toHaveBeenCalledWith('ev1', { x: 0.5, y: 0.55 }));
    expect(api.uploadImage).not.toHaveBeenCalled();
    expect(api.deleteImage).not.toHaveBeenCalled();
  });

  it('does not touch an unchanged image', async () => {
    const api = fakeApi({
      getEvent: vi.fn().mockResolvedValue(eventFixture({ cover: { url: '/img?v=1', focus: { x: 0.5, y: 0.5 } } })),
    });
    renderWithApp(<EventEditor today={today} />, { api, route: '/e/ev1/edit', path: '/e/:id/edit' });
    await screen.findByTestId('cover-frame');
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    await screen.findByTestId('elsewhere');
    expect(api.setImageFocus).not.toHaveBeenCalled();
    expect(api.uploadImage).not.toHaveBeenCalled();
  });
});
