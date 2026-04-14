import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import RetryJobs from './RetryJobs';

function genJobs(n) {
  return Array.from({ length: n }, (_, i) => ({
    id: i + 1,
    name: 'test',
    args: {},
    t: 1467760821,
    err: 'err',
  }));
}

describe('RetryJobs', () => {
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
            { id: 1, name: 'test', args: {}, t: 1467760821, err: 'err1' },
            { id: 2, name: 'test2', args: {}, t: 1467760822, err: 'err2' },
          ],
        }),
    });

    render(<RetryJobs url="./retry_jobs" />);

    expect(await screen.findByText('2 job(s) scheduled to be retried.')).toBeInTheDocument();
    expect(screen.getByText('test')).toBeInTheDocument();
    expect(screen.getByText('test2')).toBeInTheDocument();
  });

  it('navigates to next page when page link is clicked', async () => {
    global.fetch.mockResolvedValue({
      json: () => Promise.resolve({ count: 21, jobs: genJobs(21) }),
    });

    const user = userEvent.setup();
    render(<RetryJobs url="./retry_jobs" />);

    await screen.findByText('21 job(s) scheduled to be retried.');

    // Click page 2 link
    await user.click(screen.getByText('2'));

    expect(global.fetch).toHaveBeenLastCalledWith('./retry_jobs?page=2');
  });
});
