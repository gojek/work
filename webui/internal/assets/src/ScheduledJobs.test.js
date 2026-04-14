import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ScheduledJobs from './ScheduledJobs';

function genJobs(n) {
  return Array.from({ length: n }, (_, i) => ({
    id: i + 1,
    name: 'test',
    args: {},
    run_at: 1467760821,
    err: 'err',
  }));
}

describe('ScheduledJobs', () => {
  beforeEach(() => {
    global.fetch = jest.fn();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('shows jobs', async () => {
    global.fetch.mockResolvedValueOnce({
      json: () =>
        Promise.resolve({
          count: 2,
          jobs: [
            { id: 1, name: 'test', args: {}, run_at: 1467760821 },
            { id: 2, name: 'test2', args: {}, run_at: 1467760822 },
          ],
        }),
    });

    render(<ScheduledJobs url="./scheduled_jobs" />);

    expect(await screen.findByText('2 job(s) scheduled.')).toBeInTheDocument();
    expect(screen.getByText('test')).toBeInTheDocument();
    expect(screen.getByText('test2')).toBeInTheDocument();
  });

  it('navigates to next page when page link is clicked', async () => {
    global.fetch.mockResolvedValue({
      json: () => Promise.resolve({ count: 21, jobs: genJobs(21) }),
    });

    const user = userEvent.setup();
    render(<ScheduledJobs url="./scheduled_jobs" />);

    await screen.findByText('21 job(s) scheduled.');

    await user.click(screen.getByText('2'));

    expect(global.fetch).toHaveBeenLastCalledWith('./scheduled_jobs?page=2');
  });
});
