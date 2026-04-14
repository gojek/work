import React, { useState } from 'react';
import PropTypes from 'prop-types';
import PageList from './PageList';
import UnixTime from './UnixTime';
import Args from './Args';
import usePaginatedFetch from './usePaginatedFetch';

export default function DeadJobs({ fetchURL, deleteURL, deleteAllURL, retryURL, retryAllURL }) {
  const { page, setPage, count, jobs, refresh } = usePaginatedFetch(fetchURL);
  const [selected, setSelected] = useState([]);

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
                  <input type="checkbox" checked={selected.length > 0} onChange={checkAll} />
                </th>
                <th>Name</th>
                <th>Arguments</th>
                <th>Error</th>
                <th>Died At</th>
              </tr>
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
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
      <div className="btn-group" role="group">
        <button type="button" className="btn btn-default" onClick={deleteSelected}>
          Delete Selected Jobs
        </button>
        <button type="button" className="btn btn-default" onClick={retrySelected}>
          Retry Selected Jobs
        </button>
        <button type="button" className="btn btn-default" onClick={deleteAll}>
          Delete All Jobs
        </button>
        <button type="button" className="btn btn-default" onClick={retryAll}>
          Retry All Jobs
        </button>
      </div>
    </div>
  );
}

DeadJobs.propTypes = {
  fetchURL: PropTypes.string,
  deleteURL: PropTypes.string,
  deleteAllURL: PropTypes.string,
  retryURL: PropTypes.string,
  retryAllURL: PropTypes.string,
};
