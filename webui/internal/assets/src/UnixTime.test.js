import React from 'react';
import { render } from '@testing-library/react';
import UnixTime from './UnixTime';

describe('UnixTime', () => {
  it('formats human-readable time string', () => {
    const { container } = render(<UnixTime ts={1467753603} />);
    const timeEl = container.querySelector('time');

    expect(timeEl).toHaveAttribute('dateTime', '2016-07-05T21:20:03.000Z');
    expect(timeEl).toHaveTextContent('2016/07/05 21:20:03');
  });
});
