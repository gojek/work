import { useState, useEffect, useCallback } from 'react';

export default function usePaginatedFetch(url) {
  const [page, setPage] = useState(1);
  const [count, setCount] = useState(0);
  const [jobs, setJobs] = useState([]);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    if (!url) return;
    fetch(`${url}?page=${page}`)
      .then((resp) => resp.json())
      .then((data) => {
        setCount(data.count);
        setJobs(data.jobs || []);
      });
  }, [url, page, refreshKey]);

  const refresh = useCallback(() => setRefreshKey((k) => k + 1), []);

  return { page, setPage, count, jobs, refresh };
}
