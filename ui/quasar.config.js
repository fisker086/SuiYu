/* eslint-env node */

const { configure } = require('quasar/wrappers')
const path = require('path')

// Dev: proxy /api → Go API. Must match SERVER_PORT (default 8080).
// Use localhost (not 127.0.0.1) as default: avoids Node EADDRNOTAVAIL on some macOS/Node versions with IPv4-only binds.
// Override: AGENTSPHERE_API_PROXY=http://127.0.0.1:9000
const apiProxyTarget = process.env.AGENTSPHERE_API_PROXY || 'http://localhost:8080'

module.exports = configure(function (ctx) {
  return {
    boot: [
      'i18n',
      'axios'
    ],

    css: [
      'app.sass'
    ],

    extras: [
      'material-icons'
    ],

    build: {
      target: {
        browser: ['es2019', 'edge88', 'firefox78', 'chrome87', 'safari13.1'],
        node: 'node20'
      },

      vueRouterMode: 'history',

      env: {
        // Production: empty → axios uses same-origin /api/v1 (embedded in Go binary).
        API: ctx.dev
          ? ''
          : (process.env.AGENTSPHERE_PUBLIC_API || '')
      },

      extendViteConf(viteConf, { isClient }) {
        if (isClient) {
          Object.assign(viteConf.build, {
            rollupOptions: {
              output: {
                manualChunks: {
                  vue: ['vue'],
                  vueRouter: ['vue-router'],
                  axios: ['axios']
                }
              }
            }
          })
        }
      },

      vitePlugins: [
        ['@intlify/vite-plugin-vue-i18n', {
          include: path.resolve(__dirname, './src/i18n/**')
        }],
        ['vite-plugin-checker', {
          vueTsc: {
            tsconfigPath: 'tsconfig.vue-tsc.json'
          },
          eslint: {
            lintCommand: 'eslint "./**/*.{js,ts,mjs,cjs,vue}"'
          }
        }, { server: false }]
      ]
    },

    devServer: {
      host: 'localhost',
      port: 9000,
      open: true,
      proxy: {
        '/api': {
          target: apiProxyTarget,
          // Same-machine Go API: false avoids odd Host/IPv4 issues with some Node proxy stacks
          changeOrigin: false,
          secure: false,
          ws: true
        }
      }
    },

    framework: {
      config: {
        notify: {
          position: 'top',
          timeout: 2500
        }
      },

      lang: 'zh-CN',

      plugins: [
        'Notify',
        'Dialog',
        'Loading',
        'LocalStorage'
      ]
    },

    animations: []
  }
})
