import React from 'react';
import PropTypes from 'prop-types';
import ReactJson from 'react-json-view';

const base64regex = /^([\da-zA-Z+/]{4})*(([\da-zA-Z+/]{2}==)|([\da-zA-Z+/]{3}=))?$/;

export default class Args extends React.Component {
  static propTypes = {
    args: PropTypes.object.isRequired,
  }

  render() {
    let args = this.props.args;

    // If object has base64 encoded JSON property named 'payload', try to unwrap it.
    if (args.hasOwnProperty('payload') && base64regex.test(args.payload)) {
      try {
        args.payload = JSON.parse(atob(args.payload));
        if (Object.keys(args).length === 1) {
          args = args.payload;
        }
      } catch(e) {
        args.payload = {
          parse_error: 'not a valid JSON',
          value: args.payload
        };
      }
    }

    return (
      <ReactJson src={args}
        name={false}
        collapsed={1}
        collapseStringsAfterLength={32}
        displayDataTypes={false}
      />
    );
  }
}
