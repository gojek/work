import React, { useCallback, useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import ConfirmModal from './ConfirmModal';

// Admin mutation calls. The browser handles Basic Auth natively (server replies with
// WWW-Authenticate on 401), so we only need credentials: 'include' here.
function adminFetch(url, init = {}) {
  return fetch(url, {
    ...init,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(init.headers || {}) },
  });
}

// Inline editor for max_concurrency. Save calls PUT and replaces the local row on success.
function MaxConcurrencyCell({ value, onSave }) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(String(value));

  useEffect(() => {
    setDraft(String(value));
  }, [value]);

  if (!editing) {
    return (
      <span>
        {value}{' '}
        <button
          type="button"
          className="btn btn-link btn-xs"
          onClick={() => setEditing(true)}
          aria-label="edit max concurrency">
          edit
        </button>
      </span>
    );
  }

  function commit() {
    const n = parseInt(draft, 10);
    if (Number.isNaN(n) || n < 0) return;
    onSave(n).then((ok) => {
      if (ok) setEditing(false);
    });
  }

  return (
    <span>
      <input
        type="number"
        min="0"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        style={{ width: 80, display: 'inline-block' }}
        className="form-control input-sm"
        aria-label="max concurrency input"
      />{' '}
      <button type="button" className="btn btn-primary btn-xs" onClick={commit}>
        Save
      </button>{' '}
      <button
        type="button"
        className="btn btn-default btn-xs"
        onClick={() => {
          setEditing(false);
          setDraft(String(value));
        }}>
        Cancel
      </button>
    </span>
  );
}

MaxConcurrencyCell.propTypes = { value: PropTypes.number, onSave: PropTypes.func.isRequired };

export default function Queues({ url, adminBaseURL, adminStatusURL }) {
  const [queues, setQueues] = useState([]);
  const [confirm, setConfirm] = useState(null); // { kind, queue }
  const [adminEnabled, setAdminEnabled] = useState(false);

  useEffect(() => {
    if (!adminStatusURL) return;
    fetch(adminStatusURL)
      .then((r) => r.json())
      .then((d) => setAdminEnabled(!!d.enabled))
      .catch(() => setAdminEnabled(false));
  }, [adminStatusURL]);

  const refresh = useCallback(() => {
    if (!url) return;
    fetch(url)
      .then((resp) => resp.json())
      .then((data) => setQueues(data || []));
  }, [url]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const patchRow = useCallback((jobName, patch) => {
    setQueues((qs) => qs.map((q) => (q.job_name === jobName ? { ...q, ...patch } : q)));
  }, []);

  async function togglePause(queue) {
    const action = queue.paused ? 'resume' : 'pause';
    const resp = await adminFetch(
      `${adminBaseURL}/${encodeURIComponent(queue.job_name)}/${action}`,
      { method: 'POST' }
    );
    if (!resp.ok) return;
    const data = await resp.json();
    patchRow(queue.job_name, { paused: !!data.paused });
  }

  async function saveMaxConcurrency(queue, n) {
    const resp = await adminFetch(
      `${adminBaseURL}/${encodeURIComponent(queue.job_name)}/max_concurrency`,
      { method: 'PUT', body: JSON.stringify({ max_concurrency: n }) }
    );
    if (!resp.ok) return false;
    const data = await resp.json();
    patchRow(queue.job_name, { max_concurrency: data.max_concurrency });
    return true;
  }

  async function purgeQueue(queue) {
    const resp = await adminFetch(`${adminBaseURL}/${encodeURIComponent(queue.job_name)}/purge`, {
      method: 'POST',
    });
    if (!resp.ok) return;
    const data = await resp.json();
    patchRow(queue.job_name, { count: 0 });
    // best-effort: server already wiped, just record purged count for status
    void data;
  }

  async function resetLock(queue) {
    const resp = await adminFetch(
      `${adminBaseURL}/${encodeURIComponent(queue.job_name)}/reset_lock`,
      { method: 'POST' }
    );
    if (!resp.ok) return;
    patchRow(queue.job_name, { lock_count: 0 });
  }

  function openConfirm(kind, queue) {
    setConfirm({ kind, queue });
  }

  function closeConfirm() {
    setConfirm(null);
  }

  async function handleConfirm() {
    if (!confirm) return;
    const { kind, queue } = confirm;
    if (kind === 'purge') await purgeQueue(queue);
    if (kind === 'reset_lock') await resetLock(queue);
    closeConfirm();
  }

  const queuedCount = queues.reduce((sum, q) => sum + q.count, 0);

  return (
    <div className="panel panel-default">
      <div className="panel-heading">queues</div>
      <div className="panel-body">
        <p>
          {queues.length} queue(s) with a total of {queuedCount} item(s) queued.
        </p>
      </div>
      <div className="table-responsive">
        <table className="table">
          <tbody>
            <tr>
              <th>Name</th>
              <th>Count</th>
              <th>Latency (seconds)</th>
              <th>Lock Count</th>
              <th>Max Concurrency</th>
              <th>Paused</th>
              {adminEnabled && <th>Actions</th>}
            </tr>
            {queues.map((queue) => (
              <tr key={queue.job_name}>
                <td>{queue.job_name}</td>
                <td>{queue.count}</td>
                <td>{queue.latency}</td>
                <td>{queue.lock_count}</td>
                <td>
                  {adminEnabled ? (
                    <MaxConcurrencyCell
                      value={queue.max_concurrency || 0}
                      onSave={(n) => saveMaxConcurrency(queue, n)}
                    />
                  ) : (
                    queue.max_concurrency
                  )}
                </td>
                <td>
                  {queue.paused ? (
                    <span className="label label-warning">paused</span>
                  ) : (
                    <span className="label label-success">running</span>
                  )}
                </td>
                {adminEnabled && (
                  <td>
                    <div className="btn-group btn-group-xs" role="group">
                      <button
                        type="button"
                        className="btn btn-default"
                        onClick={() => togglePause(queue)}>
                        {queue.paused ? 'Resume' : 'Pause'}
                      </button>
                      <button
                        type="button"
                        className="btn btn-default"
                        onClick={() => openConfirm('reset_lock', queue)}>
                        Reset Lock
                      </button>
                      <button
                        type="button"
                        className="btn btn-danger"
                        onClick={() => openConfirm('purge', queue)}>
                        Purge
                      </button>
                    </div>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ConfirmModal
        open={!!confirm}
        title={
          confirm?.kind === 'purge'
            ? `Purge queue "${confirm.queue.job_name}"?`
            : `Reset lock for "${confirm?.queue?.job_name}"?`
        }
        message={
          confirm?.kind === 'purge' ? (
            <span>
              This deletes all <strong>{confirm.queue.count}</strong> pending job(s) in{' '}
              <code>{confirm.queue.job_name}</code>. This cannot be undone.
            </span>
          ) : (
            <span>
              This forces the lock counter for <code>{confirm?.queue?.job_name}</code> back to 0.
              Use only if the dead-pool reaper has not recovered stuck locks.
            </span>
          )
        }
        confirmText={confirm?.kind === 'purge' ? confirm.queue.job_name : null}
        confirmLabel={confirm?.kind === 'purge' ? 'Purge' : 'Reset Lock'}
        onCancel={closeConfirm}
        onConfirm={handleConfirm}
      />
    </div>
  );
}

Queues.propTypes = {
  url: PropTypes.string,
  adminBaseURL: PropTypes.string,
  adminStatusURL: PropTypes.string,
};
