import React from 'react';
import { createRoot } from 'react-dom/client';
import { HashRouter, Routes, Route, Link, Navigate, Outlet } from 'react-router-dom';
import Processes from './Processes';
import DeadJobs from './DeadJobs';
import Queues from './Queues';
import RetryJobs from './RetryJobs';
import ScheduledJobs from './ScheduledJobs';
import './bootstrap.min.css';

function App() {
  return (
    <div className="container" style={{ marginTop: 30, marginBottom: 60 }}>
      <header>
        <h1>gojek/work</h1>
      </header>
      <hr />
      <div className="row">
        <main className="col-md-10">
          <Outlet />
        </main>
        <aside className="col-md-2">
          <nav>
            <ul className="nav nav-pills nav-stacked">
              <li>
                <Link to="/processes">Processes</Link>
              </li>
              <li>
                <Link to="/queues">Queues</Link>
              </li>
              <li>
                <Link to="/retry_jobs">Retry Jobs</Link>
              </li>
              <li>
                <Link to="/scheduled_jobs">Scheduled Jobs</Link>
              </li>
              <li>
                <Link to="/dead_jobs">Dead Jobs</Link>
              </li>
            </ul>
          </nav>
        </aside>
      </div>
    </div>
  );
}

const root = createRoot(document.getElementById('app'));
root.render(
  <HashRouter>
    <Routes>
      <Route path="/" element={<App />}>
        <Route index element={<Navigate to="processes" replace />} />
        <Route
          path="processes"
          element={<Processes busyWorkerURL="./busy_workers" workerPoolURL="./worker_pools" />}
        />
        <Route
          path="queues"
          element={
            <Queues url="./queues" adminBaseURL="./queues" adminStatusURL="./admin_status" />
          }
        />
        <Route path="retry_jobs" element={<RetryJobs url="./retry_jobs" />} />
        <Route path="scheduled_jobs" element={<ScheduledJobs url="./scheduled_jobs" />} />
        <Route
          path="dead_jobs"
          element={
            <DeadJobs
              fetchURL="./dead_jobs"
              retryURL="./retry_dead_job"
              retryAllURL="./retry_all_dead_jobs"
              deleteURL="./delete_dead_job"
              deleteAllURL="./delete_all_dead_jobs"
            />
          }
        />
      </Route>
    </Routes>
  </HashRouter>
);
