import React, { useState, useEffect } from 'react';
import PropTypes from 'prop-types';
import UnixTime from './UnixTime';
import ShortList from './ShortList';
import Args from './Args';

function BusyWorkers({ worker }) {
  function parseJson(input) {
    if (input.length === 0) return null;
    try {
      return JSON.parse(input);
    } catch (e) {
      return { parse_error: 'not a valid JSON', value: input };
    }
  }

  return (
    <div className="table-responsive">
      <table className="table">
        <tbody>
          <tr>
            <th>Name</th>
            <th>Arguments</th>
            <th>Started At</th>
            <th>Check-in At</th>
            <th>Check-in</th>
          </tr>
          {worker.map((w) => (
            <tr key={w.worker_id}>
              <td>{w.job_name}</td>
              <td>
                <Args args={parseJson(w.args_json)} />
              </td>
              <td>
                <UnixTime ts={w.started_at} />
              </td>
              <td>
                <UnixTime ts={w.checkin_at} />
              </td>
              <td>{w.checkin}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

BusyWorkers.propTypes = {
  worker: PropTypes.arrayOf(
    PropTypes.shape({
      worker_id: PropTypes.string,
      job_name: PropTypes.string,
      started_at: PropTypes.number,
      checkin_at: PropTypes.number,
      checkin: PropTypes.string,
      args_json: PropTypes.string,
    })
  ).isRequired,
};

export default function Processes({ busyWorkerURL, workerPoolURL }) {
  const [busyWorker, setBusyWorker] = useState([]);
  const [workerPool, setWorkerPool] = useState([]);

  useEffect(() => {
    if (!busyWorkerURL) return;
    fetch(busyWorkerURL)
      .then((resp) => resp.json())
      .then((data) => {
        if (data) setBusyWorker(data);
      });
  }, [busyWorkerURL]);

  useEffect(() => {
    if (!workerPoolURL) return;
    fetch(workerPoolURL)
      .then((resp) => resp.json())
      .then((data) => {
        setWorkerPool(data.filter((w) => w.host !== ''));
      });
  }, [workerPoolURL]);

  const workerCount = workerPool.reduce((sum, pool) => sum + pool.worker_ids.length, 0);

  function getBusyPoolWorker(pool) {
    return busyWorker.filter((w) => pool.worker_ids.includes(w.worker_id));
  }

  return (
    <section>
      <header>Processes</header>
      <p>
        {workerPool.length} Worker process(es). {busyWorker.length} active worker(s) out of{' '}
        {workerCount}.
      </p>
      {workerPool.map((pool) => {
        const busy = getBusyPoolWorker(pool);
        return (
          <div key={pool.worker_pool_id} className="panel panel-default">
            <div className="table-responsive">
              <table className="table">
                <tbody>
                  <tr>
                    <td>
                      {pool.host}: {pool.pid}
                    </td>
                    <td>
                      Started <UnixTime ts={pool.started_at} />
                    </td>
                    <td>
                      Last Heartbeat <UnixTime ts={pool.heartbeat_at} />
                    </td>
                    <td>Concurrency {pool.concurrency}</td>
                  </tr>
                  <tr>
                    <td colSpan="4">
                      Servicing <ShortList item={pool.job_names} />.
                    </td>
                  </tr>
                  <tr>
                    <td colSpan="4">
                      {busy.length} active worker(s) and {pool.worker_ids.length - busy.length}{' '}
                      idle.
                    </td>
                  </tr>
                  <tr>
                    <td colSpan="4">
                      <div className="panel panel-default">
                        <BusyWorkers worker={busy} />
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        );
      })}
    </section>
  );
}

Processes.propTypes = { busyWorkerURL: PropTypes.string, workerPoolURL: PropTypes.string };
