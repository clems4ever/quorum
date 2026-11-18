import { MantineProvider } from '@mantine/core';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { CENTER, type CoverChange } from '../lib/cover';
import type { Focus } from '../lib/types';
import { CoverImagePicker } from './CoverImagePicker';

function Harness({
  currentUrl,
  initialFocus = CENTER,
  onChange,
  onFocus,
}: {
  currentUrl?: string;
  initialFocus?: Focus;
  onChange?: (c: CoverChange) => void;
  onFocus?: (f: Focus) => void;
}) {
  const [value, setValue] = useState<CoverChange>({ kind: 'keep' });
  const [focus, setFocus] = useState(initialFocus);
  return (
    <MantineProvider>
      <CoverImagePicker
        currentUrl={currentUrl}
        value={value}
        onChange={(c) => {
          onChange?.(c);
          setValue(c);
        }}
        focus={focus}
        onFocusChange={(f) => {
          onFocus?.(f);
          setFocus(f);
        }}
      />
    </MantineProvider>
  );
}

function file(name: string, type: string, size = 100) {
  const f = new File(['x'], name, { type });
  Object.defineProperty(f, 'size', { value: size });
  return f;
}

const choose = (container: HTMLElement, f: File) =>
  fireEvent.change(container.querySelector('input[type=file]')!, { target: { files: [f] } });

const cover = () => screen.getByRole('img', { name: 'Cover image' });

/** Simulates the image loading with the given natural size in a 1000x200 frame. */
function loadImage(width: number, height: number) {
  const img = cover();
  Object.defineProperty(img, 'naturalWidth', { value: width });
  Object.defineProperty(img, 'naturalHeight', { value: height });
  fireEvent.load(img);
  const frame = screen.getByTestId('cover-frame');
  frame.getBoundingClientRect = () => ({ width: 1000, height: 200 }) as DOMRect;
  return frame;
}

describe('CoverImagePicker', () => {
  it('picks an image from the drop zone and centers it', async () => {
    const onChange = vi.fn();
    const onFocus = vi.fn();
    const { container } = render(<Harness onChange={onChange} onFocus={onFocus} initialFocus={{ x: 0, y: 0 }} />);
    expect(screen.getByText('Drag an image here or click to choose one')).toBeInTheDocument();

    const photo = file('photo.jpg', 'image/jpeg');
    choose(container, photo);
    await waitFor(() => expect(onChange).toHaveBeenCalledWith({ kind: 'set', file: photo }));
    expect(onFocus).toHaveBeenCalledWith(CENTER);
    expect(cover()).toHaveAttribute('src', 'blob:photo.jpg');
    expect(cover()).toHaveStyle({ objectPosition: '50% 50%' });

    // Without a stored image, removing a new file just keeps "no image".
    await userEvent.click(screen.getByRole('button', { name: 'Remove' }));
    expect(onChange).toHaveBeenLastCalledWith({ kind: 'keep' });
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
  });

  it('repositions the image by dragging it', () => {
    const onFocus = vi.fn();
    render(<Harness currentUrl="/img?v=1" onFocus={onFocus} />);
    const frame = loadImage(1600, 900); // shown at 1000x562.5: 362.5px of vertical overflow

    // Moving before pressing, or with another pointer, does nothing.
    fireEvent.pointerMove(frame, { pointerId: 1, clientX: 0, clientY: 0 });
    fireEvent.pointerDown(frame, { pointerId: 1, clientX: 500, clientY: 100 });
    fireEvent.pointerMove(frame, { pointerId: 2, clientX: 0, clientY: 0 });
    expect(onFocus).not.toHaveBeenCalled();

    fireEvent.pointerMove(frame, { pointerId: 1, clientX: 500, clientY: -45 });
    expect(onFocus).toHaveBeenLastCalledWith({ x: 0.5, y: 0.9 });
    expect(cover()).toHaveStyle({ objectPosition: '50% 90%' });
    fireEvent.pointerUp(frame, { pointerId: 1 });
    fireEvent.pointerMove(frame, { pointerId: 1, clientX: 500, clientY: 500 });
    expect(onFocus).toHaveBeenCalledTimes(1);

    // A cancelled gesture stops too.
    fireEvent.pointerDown(frame, { pointerId: 3, clientX: 500, clientY: 100 });
    fireEvent.pointerCancel(frame, { pointerId: 3 });
    fireEvent.pointerMove(frame, { pointerId: 3, clientX: 500, clientY: 0 });
    expect(onFocus).toHaveBeenCalledTimes(1);
  });

  it('ignores drags until the image has loaded', () => {
    const onFocus = vi.fn();
    render(<Harness currentUrl="/img?v=1" onFocus={onFocus} />);
    const frame = screen.getByTestId('cover-frame');
    frame.getBoundingClientRect = () => ({ width: 1000, height: 200 }) as DOMRect;
    fireEvent.pointerDown(frame, { pointerId: 1, clientX: 0, clientY: 0 });
    fireEvent.pointerMove(frame, { pointerId: 1, clientX: 0, clientY: 50 });
    expect(onFocus).not.toHaveBeenCalled();
  });

  it('repositions with the arrow keys', async () => {
    const onFocus = vi.fn();
    render(<Harness currentUrl="/img?v=1" onFocus={onFocus} />);
    screen.getByTestId('cover-frame').focus();
    await userEvent.keyboard('{ArrowRight}');
    expect(onFocus).toHaveBeenLastCalledWith({ x: 0.55, y: 0.5 });
    await userEvent.keyboard('{Shift>}{ArrowUp}{/Shift}');
    expect(onFocus).toHaveBeenLastCalledWith({ x: 0.55, y: 0.3 });
    await userEvent.keyboard('a');
    expect(onFocus).toHaveBeenCalledTimes(2);
  });

  it('replaces and removes the stored image', async () => {
    const onChange = vi.fn();
    const { container } = render(
      <Harness currentUrl="/img?v=1" initialFocus={{ x: 0.2, y: 0.8 }} onChange={onChange} />,
    );
    expect(cover()).toHaveAttribute('src', '/img?v=1');
    expect(cover()).toHaveStyle({ objectPosition: '20% 80%' });

    // The replace button validates files itself.
    choose(container, file('drawing.svg', 'image/svg+xml'));
    expect(screen.getByRole('alert')).toHaveTextContent('Choose a JPEG, PNG, WebP or GIF image.');
    choose(container, file('huge.png', 'image/png', 11 * 1024 * 1024));
    expect(screen.getByRole('alert')).toHaveTextContent('larger than 10 MB');
    fireEvent.change(container.querySelector('input[type=file]')!, { target: { files: [] } });
    expect(onChange).not.toHaveBeenCalled();

    const photo = file('new.png', 'image/png');
    choose(container, photo);
    expect(onChange).toHaveBeenLastCalledWith({ kind: 'set', file: photo });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(cover()).toHaveStyle({ objectPosition: '50% 50%' });

    await userEvent.click(screen.getByRole('button', { name: 'Remove' }));
    expect(onChange).toHaveBeenLastCalledWith({ kind: 'remove' });
    expect(screen.getByText('Drag an image here or click to choose one')).toBeInTheDocument();
  });

  it('rejects dropped files that are too large or not images', async () => {
    const onChange = vi.fn();
    const { container } = render(<Harness onChange={onChange} />);
    choose(container, file('huge.png', 'image/png', 11 * 1024 * 1024));
    expect(await screen.findByRole('alert')).toHaveTextContent('larger than 10 MB');
    choose(container, file('drawing.svg', 'image/svg+xml'));
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Choose a JPEG, PNG, WebP or GIF image.'));
    expect(onChange).not.toHaveBeenCalled();
  });
});
