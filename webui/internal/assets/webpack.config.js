const path = require('path');

const BUILD_DIR = path.resolve(__dirname, 'build');
const APP_DIR = path.resolve(__dirname, 'src');

// Only element selectors need explicit safelist entries: they never appear as
// string literals in JS but Bootstrap's base font/sizing rules depend on them.
const PURGECSS_SAFELIST = ['body', 'html'];

module.exports = (env, argv) => {
  const isProd = (argv.mode || 'development') === 'production';

  // Loaders for bootstrap.min.css — plain CSS (no modules).
  // In production, postcss-loader runs first (right-to-left) and PurgeCSS
  // can auto-detect all class names from plain string literals in source.
  const bootstrapCssUse = ['style-loader', { loader: 'css-loader', options: { url: false } }];

  if (isProd) {
    bootstrapCssUse.push({
      loader: 'postcss-loader',
      options: {
        postcssOptions: {
          plugins: [
            [
              '@fullhuman/postcss-purgecss',
              {
                content: [APP_DIR + '/**/*.js', '!' + APP_DIR + '/**/*.test.js'],
                safelist: PURGECSS_SAFELIST,
              },
            ],
          ],
        },
      },
    });
  }

  return {
    mode: argv.mode || 'development',
    entry: APP_DIR + '/index.js',
    output: { path: BUILD_DIR, filename: 'work.js' },
    module: {
      rules: [
        {
          test: /\.js$/,
          loader: 'babel-loader',
          exclude: /node_modules/,
        },
        // Rule 1 — bootstrap.min.css: plain CSS, no modules, PurgeCSS in prod.
        {
          test: /bootstrap\.min\.css$/,
          use: bootstrapCssUse,
        },
        // Rule 2 — other src CSS (e.g. ShortList.css): CSS Modules, no PurgeCSS.
        {
          test: /\.css$/,
          exclude: [/node_modules/, /bootstrap\.min\.css$/],
          use: [
            'style-loader',
            {
              loader: 'css-loader',
              options: {
                modules: {
                  localIdentName: '[hash:base64:5]-[local]',
                  exportLocalsConvention: 'camelCaseOnly',
                },
                url: false,
              },
            },
          ],
        },
        // Rule 3 — node_modules CSS (e.g. react-json-view-lite): plain CSS.
        {
          test: /\.css$/,
          include: /node_modules/,
          use: ['style-loader', 'css-loader'],
        },
        {
          test: /\.(png|woff|woff2|eot|ttf|svg)(\?[#a-z_]+)?$/,
          type: 'asset/inline',
        },
      ],
    },
  };
};
