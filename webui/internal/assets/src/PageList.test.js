import React from 'react';
import { render } from '@testing-library/react';
import PageList from './PageList';

describe('PageList', () => {
  function getPageTexts(page, totalCount) {
    const { container } = render(
      <PageList page={page} perPage={2} totalCount={totalCount} jumpTo={() => () => {}} />
    );
    return Array.from(container.querySelectorAll('li')).map((li) => {
      const text = li.textContent;
      const n = parseInt(text, 10);
      return isNaN(n) ? text : n;
    });
  }

  it('lists pages', () => {
    expect(getPageTexts(1, 13)).toEqual([1, 2, '..', 7]);
    expect(getPageTexts(2, 13)).toEqual([1, 2, 3, '..', 7]);
    expect(getPageTexts(3, 13)).toEqual([1, 2, 3, 4, '..', 7]);
    expect(getPageTexts(4, 13)).toEqual([1, '..', 3, 4, 5, '..', 7]);
    expect(getPageTexts(5, 13)).toEqual([1, '..', 4, 5, 6, 7]);
    expect(getPageTexts(6, 13)).toEqual([1, '..', 5, 6, 7]);
    expect(getPageTexts(7, 13)).toEqual([1, '..', 6, 7]);
  });

  it('renders nothing if there is nothing', () => {
    const { container } = render(
      <PageList page={1} perPage={2} totalCount={0} jumpTo={() => () => {}} />
    );
    expect(container.firstChild).toBeNull();
  });
});
