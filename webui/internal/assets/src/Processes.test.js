import React from 'react';
import { render, screen } from '@testing-library/react';
import Processes from './Processes';

const busyWorkerData = [
  {
    worker_id: '2',
    job_name: 'job1',
    started_at: 1467753603,
    checkin_at: 1467753603,
    checkin: '123',
    args_json: '{}',
  },
];

const workerPoolData = [
  {
    worker_pool_id: '1',
    started_at: 1467753603,
    heartbeat_at: 1467753603,
    job_names: ['job1', 'job2', 'job3', 'job4'],
    concurrency: 10,
    host: 'web51',
    pid: 123,
    worker_ids: ['1', '2', '3'],
  },
];

describe('Processes', () => {
  beforeEach(() => {
    global.fetch = jest.fn();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('shows workers', async () => {
    global.fetch
      .mockResolvedValueOnce({ json: () => Promise.resolve(busyWorkerData) })
      .mockResolvedValueOnce({ json: () => Promise.resolve(workerPoolData) });

    render(<Processes busyWorkerURL="./busy_workers" workerPoolURL="./worker_pools" />);

    expect(
      await screen.findByText('1 Worker process(es). 1 active worker(s) out of 3.')
    ).toBeInTheDocument();

    expect(screen.getByText('web51: 123')).toBeInTheDocument();
    expect(screen.getByText('1 active worker(s) and 2 idle.')).toBeInTheDocument();
    expect(screen.getAllByText('job1').length).toBeGreaterThan(0);
  });
});
