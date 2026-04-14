import React, { useState, useEffect } from 'react';
import PropTypes from 'prop-types';

export default function Queues({ url }) {
  const [queues, setQueues] = useState([]);

  useEffect(() => {
    if (!url) return;
    fetch(url)
      .then((resp) => resp.json())
      .then((data) => setQueues(data));
  }, [url]);

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
            </tr>
            {queues.map((queue) => (
              <tr key={queue.job_name}>
                <td>{queue.job_name}</td>
                <td>{queue.count}</td>
                <td>{queue.latency}</td>
                <td>{queue.lock_count}</td>
                <td>{queue.max_concurrency}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

Queues.propTypes = { url: PropTypes.string };
