import React from 'react';
import { render, screen } from '@testing-library/react';
import Args from './Args';

// react-json-view-lite renders data as a JSON tree; we just need to verify
// it receives the correct data. We use a simple mock so tests aren't coupled
// to the library's internal DOM structure.
jest.mock('react-json-view-lite', () => ({
  JsonView: ({ data }) => <pre data-testid="json-view">{JSON.stringify(data)}</pre>,
}));

describe('Args', () => {
  test('renders "null" span when args is null', () => {
    render(<Args args={null} />);
    expect(screen.getByText('null')).toBeInTheDocument();
  });

  test('renders a basic object without modification', () => {
    const args = { foo: 'bar', count: 42 };
    render(<Args args={args} />);
    const view = screen.getByTestId('json-view');
    expect(JSON.parse(view.textContent)).toEqual(args);
  });

  test('decodes a valid base64-encoded JSON payload', () => {
    const payload = { hello: 'world' };
    const encoded = btoa(JSON.stringify(payload));
    render(<Args args={{ payload: encoded, other: 'value' }} />);
    const view = screen.getByTestId('json-view');
    expect(JSON.parse(view.textContent)).toEqual({ payload, other: 'value' });
  });

  test('unwraps single-key payload after base64 decode', () => {
    const payload = { nested: true };
    const encoded = btoa(JSON.stringify(payload));
    render(<Args args={{ payload: encoded }} />);
    const view = screen.getByTestId('json-view');
    // When args only has "payload", the component replaces args with the decoded value
    expect(JSON.parse(view.textContent)).toEqual(payload);
  });

  test('leaves payload unchanged when it is not valid base64', () => {
    const args = { payload: 'not base64!!!', other: 1 };
    render(<Args args={args} />);
    const view = screen.getByTestId('json-view');
    expect(JSON.parse(view.textContent)).toEqual(args);
  });

  test('marks payload as parse_error when base64 decodes but is not valid JSON', () => {
    const notJson = btoa('this is not json');
    render(<Args args={{ payload: notJson }} />);
    const view = screen.getByTestId('json-view');
    const rendered = JSON.parse(view.textContent);
    expect(rendered.payload.parse_error).toBe('not a valid JSON');
    expect(rendered.payload.value).toBe(notJson);
  });
});
