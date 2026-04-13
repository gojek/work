import React from 'react';
import PropTypes from 'prop-types';
import PageList from './PageList';
import UnixTime from './UnixTime';
import Args from './Args';
import usePaginatedFetch from './usePaginatedFetch';

export default function ScheduledJobs({ url }) {
  const { page, setPage, count, jobs } = usePaginatedFetch(url);

  return (
    <div className="panel panel-default">
      <div className="panel-heading">Scheduled Jobs</div>
      <div className="panel-body">
        <p>{count} job(s) scheduled.</p>
        <PageList page={page} totalCount={count} perPage={20} jumpTo={(p) => () => setPage(p)} />
      </div>
      <div className="table-responsive">
        <table className="table">
          <tbody>
            <tr>
              <th>Name</th>
              <th>Arguments</th>
              <th>Scheduled For</th>
            </tr>
            {jobs.map((job) => (
              <tr key={job.id}>
                <td>{job.name}</td>
                <td>
                  <Args args={job.args} />
                </td>
                <td>
                  <UnixTime ts={job.run_at} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

ScheduledJobs.propTypes = { url: PropTypes.string };
