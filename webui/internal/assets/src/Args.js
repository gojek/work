import React from 'react';
import PropTypes from 'prop-types';
import { JsonView, defaultStyles } from 'react-json-view-lite';
import 'react-json-view-lite/dist/index.css';

const base64regex = /^([\da-zA-Z+/]{4})*(([\da-zA-Z+/]{2}==)|([\da-zA-Z+/]{3}=))?$/;

export default function Args({ args: rawArgs }) {
  if (rawArgs == null) {
    return <span>null</span>;
  }

  let args = { ...rawArgs };

  if (Object.prototype.hasOwnProperty.call(args, 'payload') && base64regex.test(args.payload)) {
    try {
      args.payload = JSON.parse(atob(args.payload));
      if (Object.keys(args).length === 1) {
        args = args.payload;
      }
    } catch (e) {
      args.payload = { parse_error: 'not a valid JSON', value: args.payload };
    }
  }

  return <JsonView data={args} shouldExpandNode={(level) => level < 1} style={defaultStyles} />;
}

Args.propTypes = { args: PropTypes.object };
