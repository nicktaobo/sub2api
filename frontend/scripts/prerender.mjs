/**
 * Build-time prerender: 用 puppeteer 抓取公开页 SEO 快照写入 dist。
 * 输出目录与后端静态资源 serve 路径一致：
 *   /                -> dist/index.html
 *   /models          -> dist/models/index.html
 *   /docs/quickstart -> dist/docs/quickstart/index.html
 *   /docs/api-guide  -> dist/docs/api-guide/index.html
 *
 * API 请求在 prerender 期间被拦截（abort），前端会用兜底状态渲染；运行时 Vue
 * 会重新挂载并拉真实数据。
 */

import { fileURLToPath } from 'node:url'
import { dirname, resolve, join, extname } from 'node:path'
import { mkdirSync, writeFileSync, existsSync, readFileSync, statSync } from 'node:fs'
import { createServer } from 'node:http'
import puppeteer from 'puppeteer'

const __dirname = dirname(fileURLToPath(import.meta.url))
const distDir = resolve(__dirname, '..', '..', 'backend', 'internal', 'web', 'dist')

if (!existsSync(distDir)) {
  console.error(`[prerender] dist 不存在：${distDir}`)
  process.exit(1)
}

const ROUTES = ['/', '/models', '/docs/quickstart', '/docs/api-guide']

// 把 vite build 刚出的 dist/index.html 锁定到内存。所有 SPA 路由 fallback 都用这一份，
// 避免上一次 prerender 写出的（可能损坏的）<path>/index.html 影响后续路由。
const baseIndexHTML = readFileSync(join(distDir, 'index.html'), 'utf-8')

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.mjs': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
  '.map': 'application/json; charset=utf-8',
  '.txt': 'text/plain; charset=utf-8',
  '.xml': 'application/xml; charset=utf-8',
}

const server = createServer((req, res) => {
  const url = new URL(req.url, 'http://x')
  const pathname = url.pathname

  // 静态资源（带后缀且不是 .html）→ 直接从 dist serve
  const ext = extname(pathname)
  if (ext && ext !== '.html') {
    const filePath = join(distDir, pathname)
    try {
      const stat = statSync(filePath)
      if (stat.isFile()) {
        res.statusCode = 200
        res.setHeader('Content-Type', MIME[ext] || 'application/octet-stream')
        res.end(readFileSync(filePath))
        return
      }
    } catch {
      res.statusCode = 404
      res.end('not found')
      return
    }
  }

  // 所有其他请求（SPA 路由）→ 返回内存里的 baseIndexHTML
  res.statusCode = 200
  res.setHeader('Content-Type', MIME['.html'])
  res.end(baseIndexHTML)
})

await new Promise((r) => server.listen(0, r))
const { port } = server.address()
console.log(`[prerender] static server on http://localhost:${port}`)

const browser = await puppeteer.launch({
  headless: true,
  args: ['--no-sandbox', '--disable-setuid-sandbox'],
})

let failed = 0

try {
  for (const route of ROUTES) {
    const page = await browser.newPage()

    await page.setRequestInterception(true)
    page.on('request', (req) => {
      const u = req.url()
      // 拦截本地 API：dist 没这些路径，让前端走 catch 分支
      if (u.startsWith(`http://localhost:${port}/`) && /\/(api|v1|setup)\//.test(u)) {
        req.abort('failed').catch(() => {})
        return
      }
      // 拦截外部 SDK（Stripe / Airwallex 等），它们在公开页用不到且会持续发起请求干扰 networkidle
      if (!u.startsWith(`http://localhost:${port}/`) && !u.startsWith('data:')) {
        req.abort('failed').catch(() => {})
        return
      }
      req.continue().catch(() => {})
    })

    page.on('pageerror', (e) => console.warn(`[prerender] ${route} pageerror:`, e.message))

    const url = `http://localhost:${port}${route}`
    console.log(`[prerender] -> ${route}`)
    try {
      await page.goto(url, { waitUntil: 'networkidle0', timeout: 30_000 })
    } catch (e) {
      console.error(`[prerender] goto 失败 ${route}: ${e.message}`)
      failed++
      await page.close()
      continue
    }

    // 等 Vue mount 完成（#app 有 children）。main.ts await initI18n() + router.isReady()
    // 是 promise 链，加上 i18n 异步 loadLocaleMessages，需要给足时间。
    try {
      await page.waitForFunction(
        () => (document.querySelector('#app')?.children.length ?? 0) > 0,
        { timeout: 30_000, polling: 100 },
      )
    } catch (e) {
      console.warn(`[prerender] ${route} #app 未渲染：${e.message}`)
      failed++
    }
    // mount 之后多给一帧让 useHead 的 head entries 写入 document.head
    await new Promise((r) => setTimeout(r, 500))

    const html = await page.content()
    await page.close()

    const outPath =
      route === '/'
        ? join(distDir, 'index.html')
        : join(distDir, route.replace(/^\//, ''), 'index.html')
    mkdirSync(dirname(outPath), { recursive: true })
    writeFileSync(outPath, html, 'utf-8')
    console.log(`[prerender] wrote ${outPath} (${html.length} bytes)`)
  }
} finally {
  await browser.close()
  server.close()
}

if (failed > 0) {
  console.error(`[prerender] ${failed} route(s) failed`)
  process.exit(1)
}
console.log('[prerender] done')
