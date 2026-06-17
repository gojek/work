import React from 'react';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Queues from './Queues';

const twoQueues = [
  { job_name: 'test', count: 1, latency: 0, lock_count: 1, max_concurrency: 10, paused: false },
  { job_name: 'test2', count: 2, latency: 0, lock_count: 2, max_concurrency: 0, paused: true },
];

function mockList(extra) {
  global.fetch = jest.fn((url) => {
    if (url === './queues') {
      return Promise.resolve({ json: () => Promise.resolve(twoQueues) });
    }
    return Promise.resolve({ ok: true, json: () => Promise.resolve(extra || {}) });
  });
}

describe('Queues', () => {
  afterEach(() => jest.restoreAllMocks());

  it('renders rows with paused/running labels and action buttons', async () => {
    mockList();
    render(<Queues url="./queues" />);

    await screen.findByText('test');
    expect(screen.getByText('running')).toBeInTheDocument();
    expect(screen.getByText('paused')).toBeInTheDocument();

    // Controls are always present (no admin gating).
    expect(screen.getByText('Resume')).toBeInTheDocument(); // test2 is paused
    expect(screen.getAllByText('Pause')).toHaveLength(1); // test is running
    expect(screen.getAllByText('Reset Lock')).toHaveLength(2);
    expect(screen.getAllByText('Purge')).toHaveLength(2);
  });

  it('pauses a running queue via POST', async () => {
    mockList({ paused: true });
    const user = userEvent.setup();
    render(<Queues url="./queues" />);
    await screen.findByText('test');

    await user.click(screen.getByText('Pause'));

    expect(global.fetch).toHaveBeenCalledWith(
      './queues/test/pause',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('resumes a paused queue via POST', async () => {
    mockList({ paused: false });
    const user = userEvent.setup();
    render(<Queues url="./queues" />);
    await screen.findByText('test2');

    await user.click(screen.getByText('Resume'));

    expect(global.fetch).toHaveBeenCalledWith(
      './queues/test2/resume',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('edits max concurrency via PUT', async () => {
    mockList({ max_concurrency: 5 });
    const user = userEvent.setup();
    render(<Queues url="./queues" />);
    await screen.findByText('test');

    await user.click(screen.getAllByLabelText('edit max concurrency')[0]);
    const input = screen.getByLabelText('max concurrency input');
    await user.clear(input);
    await user.type(input, '5');
    await user.click(screen.getByText('Save'));

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        './queues/test/max_concurrency',
        expect.objectContaining({ method: 'PUT', body: JSON.stringify({ max_concurrency: 5 }) })
      );
    });
  });

  it('purges a queue only after typed confirmation', async () => {
    mockList();
    const user = userEvent.setup();
    render(<Queues url="./queues" />);
    await screen.findByText('test');

    await user.click(screen.getAllByText('Purge')[0]);

    const dialog = screen.getByRole('dialog');
    const confirmBtn = within(dialog).getByRole('button', { name: /^Purge$/ });
    expect(confirmBtn).toBeDisabled();

    await user.type(within(dialog).getByLabelText(/Type .* to confirm/i), 'test');
    expect(confirmBtn).toBeEnabled();
    await user.click(confirmBtn);

    expect(global.fetch).toHaveBeenCalledWith(
      './queues/test/purge',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('resets lock after confirmation', async () => {
    mockList();
    const user = userEvent.setup();
    render(<Queues url="./queues" />);
    await screen.findByText('test');

    await user.click(screen.getAllByText('Reset Lock')[0]);
    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: /Reset Lock/ }));

    expect(global.fetch).toHaveBeenCalledWith(
      './queues/test/reset_lock',
      expect.objectContaining({ method: 'POST' })
    );
  });
});
