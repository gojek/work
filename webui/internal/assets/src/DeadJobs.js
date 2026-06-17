import React, { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import PageList from './PageList';
import UnixTime from './UnixTime';
import Args from './Args';
import ConfirmModal from './ConfirmModal';
import usePaginatedFetch from './usePaginatedFetch';

export default function DeadJobs({ fetchURL, deleteURL, deleteAllURL, retryURL, retryAllURL }) {
  const { page, setPage, count, jobs, refresh } = usePaginatedFetch(fetchURL);

  const [selected, setSelected] = useState([]);
  const [bulkName, setBulkName] = useState('');
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkResult, setBulkResult] = useState(null);
  const [bulkConfirm, setBulkConfirm] = useState(null);

  const nameHints = useMemo(() => [...new Set(jobs.map((j) => j.name))], [jobs]);
  const bulkAvailable = Boolean(deleteAllURL || retryAllURL);
  const rowActionsAvailable = Boolean(deleteURL || retryURL);
  const trimmedBulkName = bulkName.trim();
  const bulkActionsDisabled = bulkBusy || !trimmedBulkName;

  function checked(job) {
    return selected.includes(job);
  }

  function check(job) {
    const index = selected.indexOf(job);
    if (index >= 0) {
      setSelected(selected.filter((_, i) => i !== index));
    } else {
      setSelected([...selected, job]);
    }
  }

  function checkAll() {
    if (selected.length > 0) {
      setSelected([]);
    } else {
      setSelected([...jobs]);
    }
  }

  function deleteAll() {
    if (!deleteAllURL) return;
    fetch(deleteAllURL, { method: 'post' }).then(() => {
      setSelected([]);
      setPage(1);
      refresh();
    });
  }

  function deleteSelected() {
    const promises = selected.map((job) => {
      if (!deleteURL) return Promise.resolve();
      return fetch(`${deleteURL}/${job.died_at}/${job.id}`, { method: 'post' });
    });
    Promise.all(promises).then(() => {
      setSelected([]);
      refresh();
    });
  }

  function retryAll() {
    if (!retryAllURL) return;
    fetch(retryAllURL, { method: 'post' }).then(() => {
      setSelected([]);
      setPage(1);
      refresh();
    });
  }

  function retrySelected() {
    const promises = selected.map((job) => {
      if (!retryURL) return Promise.resolve();
      return fetch(`${retryURL}/${job.died_at}/${job.id}`, { method: 'post' });
    });
    Promise.all(promises).then(() => {
      setSelected([]);
      refresh();
    });
  }

  function deleteOne(job) {
    if (!deleteURL) return;
    fetch(`${deleteURL}/${job.died_at}/${job.id}`, { method: 'post' }).then(() => {
      setSelected((s) => s.filter((j) => j !== job));
      refresh();
    });
  }

  function retryOne(job) {
    if (!retryURL) return;
    fetch(`${retryURL}/${job.died_at}/${job.id}`, { method: 'post' }).then(() => {
      setSelected((s) => s.filter((j) => j !== job));
      refresh();
    });
  }

  function openBulkConfirm(action) {
    if (!trimmedBulkName) return;
    setBulkConfirm({ action, name: trimmedBulkName });
  }

  function closeBulkConfirm() {
    setBulkConfirm(null);
  }

  function submitBulk(action, name) {
    const baseURL = action === 'delete' ? deleteAllURL : retryAllURL;
    if (!baseURL) return;
    setBulkBusy(true);
    setBulkResult(null);
    fetch(`${baseURL}?job_name=${encodeURIComponent(name)}`, { method: 'post' })
      .then(async (resp) => {
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) {
          throw new Error(data.error || `request failed (${resp.status})`);
        }
        const n = action === 'delete' ? data.deleted : data.retried;
        const verb = action === 'delete' ? 'Deleted' : 'Retried';
        setBulkResult({
          ok: true,
          message: `${verb} ${n != null ? n : 0} dead job(s) named "${name}".`,
        });
      })
      .catch((err) => {
        setBulkResult({ ok: false, message: String(err.message || err) });
      })
      .finally(() => {
        setBulkBusy(false);
        setBulkName('');
        setPage(1);
        refresh();
      });
  }

  function handleBulkConfirm() {
    if (!bulkConfirm) return;
    const { action, name } = bulkConfirm;
    closeBulkConfirm();
    submitBulk(action, name);
  }

  const hasSelection = selected.length > 0;
  const selectedSuffix = hasSelection ? ` (${selected.length})` : '';
  const colSpan = rowActionsAvailable ? 6 : 5;

  return (
    <div>
      <div className="panel panel-default">
        <div className="panel-heading">Dead Jobs</div>
        <div className="panel-body">
          <p>{count} job(s) are dead.</p>
          <PageList page={page} totalCount={count} perPage={20} jumpTo={(p) => () => setPage(p)} />
        </div>
        <div className="table-responsive">
          <table className="table">
            <tbody>
              <tr>
                <th>
                  <input type="checkbox" checked={hasSelection} onChange={checkAll} />
                </th>
                <th>Name</th>
                <th>Arguments</th>
                <th>Error</th>
                <th>Died At</th>
                {rowActionsAvailable && <th />}
              </tr>
              {jobs.length === 0 && (
                <tr>
                  <td colSpan={colSpan} className="text-center text-muted" style={{ padding: 24 }}>
                    No dead jobs.
                  </td>
                </tr>
              )}
              {jobs.map((job) => (
                <tr key={job.id}>
                  <td>
                    <input type="checkbox" checked={checked(job)} onChange={() => check(job)} />
                  </td>
                  <td>{job.name}</td>
                  <td>
                    <Args args={job.args} />
                  </td>
                  <td>{job.err}</td>
                  <td>
                    <UnixTime ts={job.t} />
                  </td>
                  {rowActionsAvailable && (
                    <td style={{ whiteSpace: 'nowrap', textAlign: 'right' }}>
                      <div className="btn-group btn-group-xs" role="group">
                        {retryURL && (
                          <button
                            type="button"
                            className="btn btn-default"
                            onClick={() => retryOne(job)}>
                            Retry
                          </button>
                        )}
                        {deleteURL && (
                          <button
                            type="button"
                            className="btn btn-default"
                            onClick={() => deleteOne(job)}>
                            Delete
                          </button>
                        )}
                      </div>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="panel-footer">
          <div className="btn-group" role="group">
            <button
              type="button"
              className="btn btn-default"
              disabled={!hasSelection}
              onClick={deleteSelected}>
              Delete Selected{selectedSuffix}
            </button>
            <button
              type="button"
              className="btn btn-default"
              disabled={!hasSelection}
              onClick={retrySelected}>
              Retry Selected{selectedSuffix}
            </button>
            <button type="button" className="btn btn-default" onClick={deleteAll}>
              Delete All
            </button>
            <button type="button" className="btn btn-default" onClick={retryAll}>
              Retry All
            </button>
          </div>

          {bulkAvailable && (
            <div style={bulkBoxStyle}>
              <strong>Delete or retry by job name</strong>
              <p className="text-muted" style={{ margin: '4px 0 10px' }}>
                Acts on every dead job with the exact name you enter.
              </p>
              <input
                type="text"
                className="form-control"
                placeholder="e.g. foobar"
                list="dead-job-name-hints"
                aria-label="Job name for bulk delete or retry"
                value={bulkName}
                onChange={(e) => setBulkName(e.target.value)}
                disabled={bulkBusy}
                style={{
                  display: 'inline-block',
                  width: 280,
                  marginRight: 8,
                  verticalAlign: 'top',
                }}
              />
              {retryAllURL && (
                <button
                  type="button"
                  className="btn btn-default"
                  disabled={bulkActionsDisabled}
                  onClick={() => openBulkConfirm('retry')}>
                  Retry matching
                </button>
              )}{' '}
              {deleteAllURL && (
                <button
                  type="button"
                  className="btn btn-danger"
                  disabled={bulkActionsDisabled}
                  onClick={() => openBulkConfirm('delete')}>
                  Delete matching
                </button>
              )}
              <datalist id="dead-job-name-hints">
                {nameHints.map((name) => (
                  <option key={name} value={name} />
                ))}
              </datalist>
              {bulkResult && bulkResult.ok && (
                <div className="alert alert-success" style={alertStyle} role="status">
                  {bulkResult.message}
                </div>
              )}
              {bulkResult && !bulkResult.ok && (
                <div className="alert alert-danger" style={alertStyle} role="status">
                  {bulkResult.message}
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <ConfirmModal
        open={!!bulkConfirm}
        title={
          bulkConfirm?.action === 'delete'
            ? `Delete dead jobs named "${bulkConfirm?.name}"?`
            : `Retry dead jobs named "${bulkConfirm?.name}"?`
        }
        message={
          bulkConfirm?.action === 'delete' ? (
            <span>
              This permanently removes every dead job named <code>{bulkConfirm?.name}</code> from
              the dead queue.
            </span>
          ) : (
            <span>
              This re-enqueues every dead job named <code>{bulkConfirm?.name}</code> onto the work
              queue.
            </span>
          )
        }
        confirmText={bulkConfirm?.action === 'delete' ? bulkConfirm?.name : null}
        confirmLabel={bulkConfirm?.action === 'delete' ? 'Delete' : 'Retry'}
        onCancel={closeBulkConfirm}
        onConfirm={handleBulkConfirm}
      />
    </div>
  );
}

const alertStyle = { marginTop: 12, marginBottom: 0, maxWidth: 560 };

const bulkBoxStyle = {
  marginTop: 16,
  padding: 16,
  background: '#f5f5f5',
  border: '1px solid #e3e3e3',
  borderRadius: 4,
};

DeadJobs.propTypes = {
  fetchURL: PropTypes.string,
  deleteURL: PropTypes.string,
  deleteAllURL: PropTypes.string,
  retryURL: PropTypes.string,
  retryAllURL: PropTypes.string,
};
