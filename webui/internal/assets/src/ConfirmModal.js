import React, { useState } from 'react';
import PropTypes from 'prop-types';

// Inline overlay styles avoid pulling in Bootstrap's JS modal dependency.
const overlayStyle = {
  position: 'fixed',
  inset: 0,
  background: 'rgba(0,0,0,0.4)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  zIndex: 1050,
};

const dialogStyle = {
  background: 'white',
  borderRadius: 4,
  minWidth: 360,
  maxWidth: 520,
  padding: '20px 24px',
  boxShadow: '0 5px 15px rgba(0,0,0,0.5)',
};

// ConfirmModal is a typed-name confirmation modal. The Confirm button is disabled until
// the user types `confirmText` exactly, modeled after GitHub's destructive-action UX.
// When `confirmText` is null/undefined/empty, no typing is required (simple yes/no).
export default function ConfirmModal({
  open,
  title,
  message,
  confirmText,
  confirmLabel,
  onConfirm,
  onCancel,
}) {
  const [typed, setTyped] = useState('');
  // Reset typed text on each open transition. Tracking previous `open` via state and
  // setting during render is React's documented pattern for derived state.
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (open) setTyped('');
  }

  if (!open) return null;

  const requireType = !!confirmText;
  const canConfirm = !requireType || typed === confirmText;

  return (
    <div role="dialog" aria-modal="true" style={overlayStyle}>
      <div style={dialogStyle}>
        <h4 style={{ marginTop: 0 }}>{title}</h4>
        <div style={{ marginBottom: 12 }}>{message}</div>
        {requireType && (
          <div className="form-group">
            <label htmlFor="confirm-input">
              Type <code>{confirmText}</code> to confirm:
            </label>
            <input
              id="confirm-input"
              type="text"
              className="form-control"
              autoFocus
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
            />
          </div>
        )}
        <div style={{ marginTop: 16, textAlign: 'right' }}>
          <button type="button" className="btn btn-default" onClick={onCancel}>
            Cancel
          </button>{' '}
          <button
            type="button"
            className="btn btn-danger"
            disabled={!canConfirm}
            onClick={onConfirm}>
            {confirmLabel || 'Confirm'}
          </button>
        </div>
      </div>
    </div>
  );
}

ConfirmModal.propTypes = {
  open: PropTypes.bool.isRequired,
  title: PropTypes.string.isRequired,
  message: PropTypes.node,
  confirmText: PropTypes.string,
  confirmLabel: PropTypes.string,
  onConfirm: PropTypes.func.isRequired,
  onCancel: PropTypes.func.isRequired,
};
