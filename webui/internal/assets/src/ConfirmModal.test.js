import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ConfirmModal from './ConfirmModal';

describe('ConfirmModal', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <ConfirmModal open={false} title="t" onConfirm={() => {}} onCancel={() => {}} />
    );
    expect(container.firstChild).toBeNull();
  });

  it('disables the Confirm button until typed text matches', async () => {
    const user = userEvent.setup();
    const onConfirm = jest.fn();
    render(
      <ConfirmModal
        open
        title="Purge alpha"
        message="really?"
        confirmText="alpha"
        confirmLabel="Purge"
        onConfirm={onConfirm}
        onCancel={() => {}}
      />
    );

    const button = screen.getByRole('button', { name: 'Purge' });
    expect(button).toBeDisabled();

    const input = screen.getByLabelText(/Type/i);
    await user.type(input, 'wrong');
    expect(button).toBeDisabled();

    await user.clear(input);
    await user.type(input, 'alpha');
    expect(button).not.toBeDisabled();

    await user.click(button);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('calls onCancel when Cancel is clicked', async () => {
    const user = userEvent.setup();
    const onCancel = jest.fn();
    render(<ConfirmModal open title="t" onConfirm={() => {}} onCancel={onCancel} />);
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('does not require typing when confirmText is empty (simple yes/no)', async () => {
    const user = userEvent.setup();
    const onConfirm = jest.fn();
    render(<ConfirmModal open title="t" onConfirm={onConfirm} onCancel={() => {}} />);
    const button = screen.getByRole('button', { name: 'Confirm' });
    expect(button).not.toBeDisabled();
    await user.click(button);
    expect(onConfirm).toHaveBeenCalled();
  });
});
