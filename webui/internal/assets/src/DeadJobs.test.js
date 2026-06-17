import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import DeadJobs from './DeadJobs';

const twoJobs = [
  { id: 1, name: 'test', args: {}, t: 1467760821, err: 'err1', died_at: 100 },
  { id: 2, name: 'test2', args: {}, t: 1467760822, err: 'err2', died_at: 200 },
];

function genJobs(n) {
  return Array.from({ length: n }, (_, i) => ({
    id: i + 1,
    name: 'test',
    args: {},
    t: 1467760821,
    err: 'err',
    died_at: i + 1,
  }));
}

describe('DeadJobs', () => {
  beforeEach(() => {
    global.fetch = jest.fn();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('shows dead jobs and handles checkbox selection', async () => {
    global.fetch.mockResolvedValue({ json: () => Promise.resolve({ count: 2, jobs: twoJobs }) });

    const user = userEvent.setup();
    render(<DeadJobs fetchURL="./dead_jobs" />);

    await screen.findByText('2 job(s) are dead.');

    let checkboxes = screen.getAllByRole('checkbox');
    expect(checkboxes).toHaveLength(3);
    expect(checkboxes[0]).not.toBeChecked();

    await user.click(checkboxes[0]);
    checkboxes = screen.getAllByRole('checkbox');
    expect(checkboxes[0]).toBeChecked();
    expect(checkboxes[1]).toBeChecked();
    expect(checkboxes[2]).toBeChecked();

    await user.click(checkboxes[1]);
    checkboxes = screen.getAllByRole('checkbox');
    expect(checkboxes[1]).not.toBeChecked();

    await user.click(checkboxes[0]);
    checkboxes = screen.getAllByRole('checkbox');
    expect(checkboxes.every((cb) => !cb.checked)).toBe(true);
  });

  it('renders four selection/all action buttons', async () => {
    global.fetch.mockResolvedValue({ json: () => Promise.resolve({ count: 2, jobs: twoJobs }) });

    render(<DeadJobs fetchURL="./dead_jobs" />);
    await screen.findByText('2 job(s) are dead.');

    expect(screen.getByRole('button', { name: /Delete Selected/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Retry Selected/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Delete All$/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Retry All$/ })).toBeInTheDocument();
  });

  it('navigates to next page when page link is clicked', async () => {
    global.fetch.mockResolvedValue({
      json: () => Promise.resolve({ count: 21, jobs: genJobs(21) }),
    });

    const user = userEvent.setup();
    render(<DeadJobs fetchURL="./dead_jobs" />);

    await screen.findByText('21 job(s) are dead.');

    await user.click(screen.getByText('2'));

    expect(global.fetch).toHaveBeenCalledWith('./dead_jobs?page=2');
  });

  it('shows an empty state when there are no dead jobs', async () => {
    global.fetch.mockResolvedValue({ json: () => Promise.resolve({ count: 0, jobs: [] }) });

    render(<DeadJobs fetchURL="./dead_jobs" />);

    expect(await screen.findByText('No dead jobs.')).toBeInTheDocument();
  });

  it('disables the selected-action buttons until rows are checked', async () => {
    global.fetch.mockResolvedValue({ json: () => Promise.resolve({ count: 2, jobs: twoJobs }) });

    const user = userEvent.setup();
    render(<DeadJobs fetchURL="./dead_jobs" />);
    await screen.findByText('2 job(s) are dead.');

    expect(screen.getByRole('button', { name: /Delete Selected/ })).toBeDisabled();
    expect(screen.getByRole('button', { name: /Retry Selected/ })).toBeDisabled();

    await user.click(screen.getAllByRole('checkbox')[1]);

    expect(screen.getByRole('button', { name: /Delete Selected \(1\)/ })).toBeEnabled();
    expect(screen.getByRole('button', { name: /Retry Selected \(1\)/ })).toBeEnabled();
  });

  it('supports per-row retry and delete', async () => {
    global.fetch.mockResolvedValue({ json: () => Promise.resolve({ count: 2, jobs: twoJobs }) });

    const user = userEvent.setup();
    render(
      <DeadJobs fetchURL="./dead_jobs" deleteURL="./delete_dead_job" retryURL="./retry_dead_job" />
    );
    await screen.findByText('2 job(s) are dead.');

    await user.click(screen.getAllByRole('button', { name: /^Retry$/ })[0]);
    expect(global.fetch).toHaveBeenCalledWith(
      './retry_dead_job/100/1',
      expect.objectContaining({ method: 'post' })
    );

    await user.click(screen.getAllByRole('button', { name: /^Delete$/ })[1]);
    expect(global.fetch).toHaveBeenCalledWith(
      './delete_dead_job/200/2',
      expect.objectContaining({ method: 'post' })
    );
  });

  describe('bulk by job name', () => {
    it('is hidden when no bulk URLs are provided', async () => {
      global.fetch.mockResolvedValue({ json: () => Promise.resolve({ count: 0, jobs: [] }) });
      render(<DeadJobs fetchURL="./dead_jobs" />);
      await screen.findByText('0 job(s) are dead.');

      expect(screen.queryByLabelText(/Job name for bulk delete or retry/i)).not.toBeInTheDocument();
    });

    it('disables bulk buttons until a name is typed', async () => {
      global.fetch.mockResolvedValue({ json: () => Promise.resolve({ count: 0, jobs: [] }) });
      const user = userEvent.setup();
      render(
        <DeadJobs
          fetchURL="./dead_jobs"
          deleteAllURL="./delete_all_dead_jobs"
          retryAllURL="./retry_all_dead_jobs"
        />
      );
      await screen.findByText('0 job(s) are dead.');

      const del = screen.getByRole('button', { name: /Delete matching/i });
      const retry = screen.getByRole('button', { name: /Retry matching/i });
      expect(del).toBeDisabled();
      expect(retry).toBeDisabled();

      await user.type(screen.getByLabelText(/Job name for bulk delete or retry/i), 'alpha');
      expect(del).toBeEnabled();
      expect(retry).toBeEnabled();
    });

    it('submits delete after typed confirmation in the modal', async () => {
      global.fetch.mockImplementation((url) => {
        if (url.startsWith('./delete_all_dead_jobs')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ status: 'ok', deleted: 7, job_name: 'alpha' }),
          });
        }
        return Promise.resolve({ ok: true, json: () => Promise.resolve({ count: 0, jobs: [] }) });
      });

      const user = userEvent.setup();
      render(
        <DeadJobs
          fetchURL="./dead_jobs"
          deleteAllURL="./delete_all_dead_jobs"
          retryAllURL="./retry_all_dead_jobs"
        />
      );
      await screen.findByText('0 job(s) are dead.');

      await user.type(screen.getByLabelText(/Job name for bulk delete or retry/i), 'alpha');
      await user.click(screen.getByRole('button', { name: /Delete matching/i }));
      await user.type(screen.getByLabelText(/Type .* to confirm/i), 'alpha');
      await user.click(screen.getByRole('button', { name: /^Delete$/ }));

      await waitFor(() => {
        expect(global.fetch).toHaveBeenCalledWith(
          './delete_all_dead_jobs?job_name=alpha',
          expect.objectContaining({ method: 'post' })
        );
      });

      expect(await screen.findByRole('status')).toHaveTextContent(
        /Deleted 7 dead job\(s\) named "alpha"/i
      );
      expect(screen.getByLabelText(/Job name for bulk delete or retry/i)).toHaveValue('');
    });

    it('submits retry after confirming in the modal', async () => {
      global.fetch.mockImplementation((url) => {
        if (url.startsWith('./retry_all_dead_jobs')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ status: 'ok', retried: 3, job_name: 'beta' }),
          });
        }
        return Promise.resolve({ ok: true, json: () => Promise.resolve({ count: 0, jobs: [] }) });
      });

      const user = userEvent.setup();
      render(
        <DeadJobs
          fetchURL="./dead_jobs"
          deleteAllURL="./delete_all_dead_jobs"
          retryAllURL="./retry_all_dead_jobs"
        />
      );
      await screen.findByText('0 job(s) are dead.');

      await user.type(screen.getByLabelText(/Job name for bulk delete or retry/i), 'beta');
      await user.click(screen.getByRole('button', { name: /Retry matching/i }));
      await user.click(screen.getByRole('button', { name: /^Retry$/ }));

      await waitFor(() => {
        expect(global.fetch).toHaveBeenCalledWith(
          './retry_all_dead_jobs?job_name=beta',
          expect.objectContaining({ method: 'post' })
        );
      });

      expect(await screen.findByRole('status')).toHaveTextContent(
        /Retried 3 dead job\(s\) named "beta"/i
      );
    });
  });
});
