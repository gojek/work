import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Queues from './Queues';

const baseQueues = [
  { job_name: 'test', count: 1, latency: 0, lock_count: 1, max_concurrency: 10, paused: false },
  { job_name: 'test2', count: 2, latency: 0, lock_count: 2, max_concurrency: 0, paused: true },
];

// mockFetch returns a fetch jest.fn() that responds to:
//  - GET admin_status -> {enabled: adminEnabled}
//  - GET queues       -> baseQueues (or `queues` override)
//  - everything else  -> the action response object provided
function mockFetch({ adminEnabled = false, queues = baseQueues, action } = {}) {
  return jest.fn((url, init = {}) => {
    if (url.includes('admin_status')) {
      return Promise.resolve({ json: () => Promise.resolve({ enabled: adminEnabled }) });
    }
    if (!init.method || init.method === 'GET') {
      return Promise.resolve({ json: () => Promise.resolve(queues) });
    }
    return Promise.resolve({ ok: true, json: () => Promise.resolve(action || {}) });
  });
}

describe('Queues (read-only)', () => {
  beforeEach(() => {
    global.fetch = mockFetch({ adminEnabled: false });
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('shows queue count and total queued items', async () => {
    render(<Queues url="./queues" adminStatusURL="./admin_status" />);

    expect(
      await screen.findByText('2 queue(s) with a total of 3 item(s) queued.')
    ).toBeInTheDocument();
    expect(screen.getByText('test')).toBeInTheDocument();
    expect(screen.getByText('test2')).toBeInTheDocument();
  });

  it('renders the paused badge based on the flag', async () => {
    render(<Queues url="./queues" adminStatusURL="./admin_status" />);
    await screen.findByText('test');
    expect(screen.getByText('paused')).toBeInTheDocument();
    expect(screen.getByText('running')).toBeInTheDocument();
  });

  it('does not render admin actions when admin is disabled', async () => {
    render(<Queues url="./queues" adminStatusURL="./admin_status" adminBaseURL="./queues" />);
    await screen.findByText('test');
    // Wait for any potential async render completion.
    await waitFor(() => {
      expect(screen.queryByText('Pause')).not.toBeInTheDocument();
      expect(screen.queryByText('Purge')).not.toBeInTheDocument();
    });
  });
});

describe('Queues (admin enabled)', () => {
  let user;

  beforeEach(() => {
    global.fetch = mockFetch({ adminEnabled: true });
    user = userEvent.setup();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('shows Pause/Resume/Purge buttons for each row', async () => {
    render(<Queues url="./queues" adminStatusURL="./admin_status" adminBaseURL="./queues" />);
    // Wait for the admin column to actually appear; baseQueues has one paused, one running -> one Resume + one Pause.
    await screen.findByText('Resume');
    expect(screen.getAllByText(/^(Pause|Resume)$/)).toHaveLength(2);
    expect(screen.getAllByText('Purge')).toHaveLength(2);
    expect(screen.getAllByText('Reset Lock')).toHaveLength(2);
  });

  it('Purge requires typing the queue name to confirm', async () => {
    render(<Queues url="./queues" adminStatusURL="./admin_status" adminBaseURL="./queues" />);
    await screen.findByText('test');

    // Click first row's Purge -> typed-name modal opens.
    const purgeButtons = screen.getAllByText('Purge');
    await user.click(purgeButtons[0]);

    expect(screen.getByRole('dialog')).toBeInTheDocument();

    // The Purge button inside the dialog should be disabled until we type the name.
    const dialog = screen.getByRole('dialog');
    const dialogPurge = dialog.querySelector('.btn-danger');
    expect(dialogPurge).toBeDisabled();

    // Type the wrong text -> still disabled.
    const input = screen.getByLabelText(/Type/i);
    await user.type(input, 'nope');
    expect(dialogPurge).toBeDisabled();

    // Clear and type the right text.
    await user.clear(input);
    await user.type(input, 'test');
    expect(dialogPurge).not.toBeDisabled();

    await user.click(dialogPurge);

    // Verify the purge endpoint was called.
    await waitFor(() => {
      const calls = global.fetch.mock.calls.map(
        ([u, init]) => `${(init && init.method) || 'GET'} ${u}`
      );
      expect(calls).toContain('POST ./queues/test/purge');
    });
  });

  it('Pause toggle calls the pause endpoint and flips the row state', async () => {
    render(<Queues url="./queues" adminStatusURL="./admin_status" adminBaseURL="./queues" />);
    await screen.findByText('test');

    // baseQueues[0] is 'test' which is NOT paused -> button label is "Pause".
    const buttons = screen.getAllByText('Pause');
    expect(buttons.length).toBeGreaterThan(0);

    // Make the action response say paused=true so the optimistic patch shows it.
    global.fetch.mockImplementation((url, init = {}) => {
      if (url.includes('admin_status')) {
        return Promise.resolve({ json: () => Promise.resolve({ enabled: true }) });
      }
      if (!init.method || init.method === 'GET') {
        return Promise.resolve({ json: () => Promise.resolve(baseQueues) });
      }
      if (url.endsWith('/test/pause')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve({ paused: true }) });
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve({}) });
    });

    await user.click(buttons[0]);

    await waitFor(() => {
      const calls = global.fetch.mock.calls.map(
        ([u, init]) => `${(init && init.method) || 'GET'} ${u}`
      );
      expect(calls).toContain('POST ./queues/test/pause');
    });
  });
});
