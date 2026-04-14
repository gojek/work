import React from 'react';
import { render, screen } from '@testing-library/react';
import ShortList from './ShortList';

describe('ShortList', () => {
  it('lists first 3 items and shows "more" for the rest', () => {
    render(<ShortList item={['1', '2', '3', '4']} />);

    expect(screen.getByText('1')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('1 more')).toBeInTheDocument();
  });
});
