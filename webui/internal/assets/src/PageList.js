import React from 'react';
import PropTypes from 'prop-types';

export default function PageList({ page, perPage, totalCount, jumpTo }) {
  const totalPage = Math.ceil(totalCount / perPage);

  function shouldShow(i) {
    if (i === 1 || i === totalPage) {
      return true;
    }
    return Math.abs(page - i) <= 1;
  }

  if (totalPage === 0) {
    return null;
  }

  const pages = [];
  for (let i = 1; i <= totalPage; i++) {
    if (i === page) {
      pages.push(
        <li key={i} className="active">
          <span>{i}</span>
        </li>
      );
    } else if (shouldShow(i)) {
      pages.push(
        <li key={i}>
          <a onClick={jumpTo(i)}>{i}</a>
        </li>
      );
    } else if (shouldShow(i - 1)) {
      pages.push(
        <li key={i} className="disabled">
          <span>..</span>
        </li>
      );
    }
  }

  return <ul className="pagination">{pages}</ul>;
}

PageList.propTypes = {
  page: PropTypes.number.isRequired,
  perPage: PropTypes.number.isRequired,
  totalCount: PropTypes.number.isRequired,
  jumpTo: PropTypes.func.isRequired,
};
