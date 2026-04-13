import React from 'react';
import PropTypes from 'prop-types';

export default function UnixTime({ ts }) {
  const t = new Date(ts * 1e3);
  return (
    <time dateTime={t.toISOString()}>
      {t.toISOString().slice(0, 19).replace(/-/g, '/').replace('T', ' ')}
    </time>
  );
}

UnixTime.propTypes = { ts: PropTypes.number.isRequired };
