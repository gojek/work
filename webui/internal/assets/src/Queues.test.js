import React from 'react';
import { render, screen } from '@testing-library/react';
import Queues from './Queues';

describe('Queues', () => {
  beforeEach(() => {
    global.fetch = jest.fn();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('shows queue count and total queued items', async () => {
    global.fetch.mockResolvedValueOnce({
      json: () =>
        Promise.resolve([
          { job_name: 'test', count: 1, latency: 0, lock_count: 1, max_concurrency: 10 },
          { job_name: 'test2', count: 2, latency: 0, lock_count: 2, max_concurrency: 0 },
        ]),
    });

    render(<Queues url="./queues" />);

    expect(
      await screen.findByText('2 queue(s) with a total of 3 item(s) queued.')
    ).toBeInTheDocument();
    expect(screen.getByText('test')).toBeInTheDocument();
    expect(screen.getByText('test2')).toBeInTheDocument();
  });
});
