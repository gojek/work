import React from 'react';
import { render, screen } from '@testing-library/react';
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
    // No action URLs provided — button clicks are no-ops so they won't re-fetch and reset selection
    render(<DeadJobs fetchURL="./dead_jobs" />);

    await screen.findByText('2 job(s) are dead.');

    let checkboxes = screen.getAllByRole('checkbox');
    expect(checkboxes).toHaveLength(3); // 1 header + 2 rows
    expect(checkboxes[0]).not.toBeChecked();
    expect(checkboxes[1]).not.toBeChecked();
    expect(checkboxes[2]).not.toBeChecked();

    // Check all via header checkbox
    await user.click(checkboxes[0]);
    checkboxes = screen.getAllByRole('checkbox'); // re-query after re-render
    expect(checkboxes[0]).toBeChecked();
    expect(checkboxes[1]).toBeChecked();
    expect(checkboxes[2]).toBeChecked();

    // Uncheck first row job
    await user.click(checkboxes[1]);
    checkboxes = screen.getAllByRole('checkbox');
    expect(checkboxes[1]).not.toBeChecked();
    expect(checkboxes[0]).toBeChecked(); // header: some selected → still truthy
    expect(checkboxes[2]).toBeChecked();

    // Re-check it
    await user.click(checkboxes[1]);
    checkboxes = screen.getAllByRole('checkbox');
    expect(checkboxes[1]).toBeChecked();

    // Uncheck all via header (selected.length > 0 → sets selected to [])
    await user.click(checkboxes[0]);
    checkboxes = screen.getAllByRole('checkbox');
    expect(checkboxes[0]).not.toBeChecked();
    expect(checkboxes[1]).not.toBeChecked();
    expect(checkboxes[2]).not.toBeChecked();
  });

  it('renders four action buttons', async () => {
    global.fetch.mockResolvedValue({ json: () => Promise.resolve({ count: 2, jobs: twoJobs }) });

    render(<DeadJobs fetchURL="./dead_jobs" />);
    await screen.findByText('2 job(s) are dead.');

    const buttons = screen.getAllByRole('button');
    expect(buttons).toHaveLength(4);
    expect(buttons[0]).toHaveTextContent('Delete Selected Jobs');
    expect(buttons[1]).toHaveTextContent('Retry Selected Jobs');
    expect(buttons[2]).toHaveTextContent('Delete All Jobs');
    expect(buttons[3]).toHaveTextContent('Retry All Jobs');
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
});
