import React from 'react';
import PropTypes from 'prop-types';
import styles from './ShortList.css';

export default function ShortList({ item }) {
  return (
    <ul className={styles.ul}>
      {item.map((el, i) => {
        if (i < 3) {
          return (
            <li key={i} className={styles.li}>
              {el}
            </li>
          );
        } else if (i === 3) {
          return (
            <li key={i} className={styles.li}>
              {item.length - 3} more
            </li>
          );
        }
        return null;
      })}
    </ul>
  );
}

ShortList.propTypes = { item: PropTypes.arrayOf(PropTypes.string).isRequired };
