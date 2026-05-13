<template>
  <div class="home-shell">
    <!-- ========== 顶部固定导航 ========== -->
    <header class="hd-nav">
      <div class="hd-nav-inner">
        <router-link to="/home" class="hd-brand">
          <div class="hd-logo-wrap">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="hd-logo-img" />
          </div>
          <span class="hd-brand-name">{{ siteName }}</span>
        </router-link>

        <div class="hd-nav-right">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="hd-icon-btn"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="hd-cta-pill"
          >
            <span class="hd-cta-avatar">{{ userInitial }}</span>
            <span>{{ t('home.dashboard') }}</span>
          </router-link>
          <router-link v-else to="/login" class="hd-cta-pill">
            {{ t('home.login') }}
          </router-link>
        </div>
      </div>
    </header>

    <!-- ========== 内容区（header 下方）========== -->
    <!-- 1) 自定义 iframe -->
    <main v-if="isHomeContentUrl" class="hd-embed-frame">
      <iframe
        :src="homeContent.trim()"
        class="h-full w-full border-0"
        allowfullscreen
      ></iframe>
    </main>

    <!-- 2) 自定义 HTML 注入 -->
    <main v-else-if="homeContent" class="hd-embed-html">
      <!-- SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
      <div v-html="homeContent"></div>
    </main>

    <!-- 3) 默认企业风首页（Apple-style） -->
    <main v-else class="hd-default">
      <!-- ===== HERO（黑色全幅） ===== -->
      <section class="hd-hero">
        <div class="hd-hero-orb"></div>
        <div class="hd-hero-orb hd-hero-orb--2"></div>
        <div class="hd-hero-grid-bg"></div>

        <div class="hd-hero-inner">
          <div class="hd-eyebrow">
            <span class="hd-eyebrow-pulse"></span>
            {{ t('home.heroSubtitle') }}
          </div>

          <h1 class="hd-hero-h1">
            <span class="hd-hero-line">All your AI.</span>
            <span class="hd-hero-line hd-hero-line--accent">One unified key.</span>
          </h1>

          <p class="hd-hero-sub">
            {{ t('home.heroDescription') }}
          </p>

          <div class="hd-hero-actions">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/register'"
              class="hd-btn-primary"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
            </router-link>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="hd-btn-text"
            >
              {{ t('home.viewDocs') }}
              <Icon name="arrowRight" size="sm" class="hd-arrow" />
            </a>
          </div>

          <!-- Hero 终端 -->
          <div class="hd-hero-terminal-wrap">
            <div class="hd-terminal">
              <div class="hd-terminal-bar">
                <span class="hd-terminal-dot hd-dot-r"></span>
                <span class="hd-terminal-dot hd-dot-y"></span>
                <span class="hd-terminal-dot hd-dot-g"></span>
                <span class="hd-terminal-title">~/sub2api</span>
              </div>
              <div class="hd-terminal-body">
                <div class="hd-code-line hd-line-1">
                  <span class="hd-code-c1">$</span>
                  <span class="hd-code-c2">curl</span>
                  <span class="hd-code-c3">https://api.sub2api.dev</span><span class="hd-code-c4">/v1/messages</span>
                </div>
                <div class="hd-code-line hd-line-2">
                  <span class="hd-code-c5">-H</span>
                  <span class="hd-code-c6">"Authorization: Bearer sk-xxx"</span>
                </div>
                <div class="hd-code-line hd-line-3">
                  <span class="hd-code-c7"># Auto-routed to optimal upstream...</span>
                </div>
                <div class="hd-code-line hd-line-4">
                  <span class="hd-code-ok">200 OK</span>
                  <span class="hd-code-resp">{ "model": "claude-opus", "content": "..." }</span>
                </div>
                <div class="hd-code-line hd-line-5">
                  <span class="hd-code-c1">$</span>
                  <span class="hd-cursor"></span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- ===== 关键数字（深色窄条） ===== -->
      <section class="hd-numbers">
        <div class="hd-section-inner">
          <div class="hd-num-grid">
            <div class="hd-num-cell">
              <div class="hd-num-v">4<small>+</small></div>
              <div class="hd-num-k">{{ t('home.providers.title') }}</div>
            </div>
            <div class="hd-num-cell">
              <div class="hd-num-v">99.9<small>%</small></div>
              <div class="hd-num-k">服务可用性</div>
            </div>
            <div class="hd-num-cell">
              <div class="hd-num-v">5<small>min</small></div>
              <div class="hd-num-k">接入耗时</div>
            </div>
            <div class="hd-num-cell">
              <div class="hd-num-v">1<small></small></div>
              <div class="hd-num-k">{{ t('home.tags.subscriptionToApi') }}</div>
            </div>
          </div>
        </div>
      </section>

      <!-- ===== Spotlight 1: 统一网关（白色全幅） ===== -->
      <section class="hd-spotlight hd-spotlight--light">
        <div class="hd-section-inner">
          <div class="hd-spot-eyebrow">UNIFIED GATEWAY</div>
          <h2 class="hd-spot-title">
            一个密钥，<br />
            <span class="hd-text-grad-blue">畅用所有主流大模型。</span>
          </h2>
          <p class="hd-spot-sub">
            无需分别申请、维护多份订阅。 Sub2API 把 Claude、GPT、Gemini 等服务整合到一套兼容标准的 API 之下，让接入变成几行代码的事。
          </p>

          <div class="hd-providers-stage">
            <div class="hd-prov-card hd-prov-claude">
              <div class="hd-prov-mark">C</div>
              <div class="hd-prov-label">Claude</div>
            </div>
            <div class="hd-prov-card hd-prov-gpt">
              <div class="hd-prov-mark">G</div>
              <div class="hd-prov-label">GPT</div>
            </div>
            <div class="hd-prov-card hd-prov-gemini">
              <div class="hd-prov-mark">G</div>
              <div class="hd-prov-label">Gemini</div>
            </div>
            <div class="hd-prov-card hd-prov-anti">
              <div class="hd-prov-mark">A</div>
              <div class="hd-prov-label">Antigravity</div>
            </div>
            <div class="hd-prov-card hd-prov-more">
              <div class="hd-prov-mark">+</div>
              <div class="hd-prov-label">{{ t('home.providers.more') }}</div>
            </div>

            <!-- 连接到中心枢纽 -->
            <div class="hd-prov-hub">
              <div class="hd-prov-hub-ring"></div>
              <div class="hd-prov-hub-core">
                <img :src="siteLogo || '/logo.png'" alt="" />
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- ===== Spotlight 2: 智能路由（黑色全幅） ===== -->
      <section class="hd-spotlight hd-spotlight--dark">
        <div class="hd-section-inner">
          <div class="hd-spot-eyebrow hd-spot-eyebrow--dark">INTELLIGENT ROUTING</div>
          <h2 class="hd-spot-title hd-spot-title--dark">
            稳定可靠的<br />
            <span class="hd-text-grad-blue-bright">智能调度引擎。</span>
          </h2>
          <p class="hd-spot-sub hd-spot-sub--dark">
            多账号池自动负载与故障切换、会话级粘性路由、按 Token 实时计费 —— 一切只为让你的每一次请求都能在最优路径上完成。
          </p>

          <!-- Bento -->
          <div class="hd-bento">
            <div class="hd-bento-tile hd-bento-1">
              <div class="hd-bento-num">01</div>
              <h3>会话保持</h3>
              <p>同一会话固定路由至同一账号，保留上下文记忆与多轮对话状态。</p>
              <div class="hd-bento-viz hd-viz-session"></div>
            </div>
            <div class="hd-bento-tile hd-bento-2">
              <div class="hd-bento-num">02</div>
              <h3>账号池调度</h3>
              <p>智能识别配额、限速、健康度，自动剔除异常账号。</p>
              <div class="hd-bento-viz hd-viz-pool"></div>
            </div>
            <div class="hd-bento-tile hd-bento-3">
              <div class="hd-bento-num">03</div>
              <h3>实时计费</h3>
              <p>按 Token 精确计量，分钟级账单更新，支持配额上限。</p>
              <div class="hd-bento-viz hd-viz-billing"></div>
            </div>
            <div class="hd-bento-tile hd-bento-4">
              <div class="hd-bento-num">04</div>
              <h3>开箱可观测</h3>
              <p>请求级日志、模型用量大盘、异常告警全部内置。</p>
              <div class="hd-bento-viz hd-viz-obs"></div>
            </div>
          </div>
        </div>
      </section>

      <!-- ===== 对比（白色全幅） ===== -->
      <section class="hd-spotlight hd-spotlight--light hd-section--tight">
        <div class="hd-section-inner">
          <div class="hd-spot-eyebrow">WHY US</div>
          <h2 class="hd-spot-title">
            两种选择，<br /><span class="hd-text-grad-blue">差距一目了然。</span>
          </h2>

          <div class="hd-compare">
            <div class="hd-cmp-col hd-cmp-col--off">
              <div class="hd-cmp-head">
                <div class="hd-cmp-tag hd-cmp-tag--off">官方订阅</div>
                <div class="hd-cmp-headline">逐家维护，逐月续费</div>
              </div>
              <ul>
                <li v-for="key in comparisonKeys" :key="key">
                  <span class="hd-cmp-cross">×</span>
                  <div>
                    <div class="hd-cmp-feat">{{ t(`home.comparison.items.${key}.feature`) }}</div>
                    <div class="hd-cmp-val">{{ t(`home.comparison.items.${key}.official`) }}</div>
                  </div>
                </li>
              </ul>
            </div>
            <div class="hd-cmp-col hd-cmp-col--us">
              <div class="hd-cmp-head">
                <div class="hd-cmp-tag hd-cmp-tag--us">本平台 · 推荐</div>
                <div class="hd-cmp-headline">一个 Key，所有可能</div>
              </div>
              <ul>
                <li v-for="key in comparisonKeys" :key="key">
                  <span class="hd-cmp-check">✓</span>
                  <div>
                    <div class="hd-cmp-feat">{{ t(`home.comparison.items.${key}.feature`) }}</div>
                    <div class="hd-cmp-val">{{ t(`home.comparison.items.${key}.us`) }}</div>
                  </div>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      <!-- ===== 最终 CTA（黑色全幅） ===== -->
      <section class="hd-final-cta">
        <div class="hd-final-orb"></div>
        <div class="hd-section-inner hd-final-inner">
          <h2 class="hd-final-title">
            {{ t('home.cta.title') }}
          </h2>
          <p class="hd-final-sub">{{ t('home.cta.description') }}</p>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/register'"
            class="hd-btn-primary hd-btn-primary--lg"
          >
            {{ isAuthenticated ? t('home.goToDashboard') : t('home.cta.button') }}
          </router-link>
        </div>
      </section>

      <!-- ===== Footer（浅色） ===== -->
      <footer class="hd-footer">
        <div class="hd-section-inner hd-footer-inner">
          <div class="hd-footer-brand">
            <div class="hd-logo-wrap hd-logo-wrap--sm">
              <img :src="siteLogo || '/logo.png'" alt="Logo" class="hd-logo-img" />
            </div>
            <span>{{ siteName }}</span>
          </div>
          <div class="hd-footer-meta">
            <span>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</span>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
            >{{ t('home.docs') }}</a>
          </div>
        </div>
      </footer>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const comparisonKeys = ['pricing', 'models', 'management', 'stability', 'control'] as const

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
/* ============ 设计 token（仅 home 内生效）============ */
.home-shell {
  --hd-brand: #0066ff;
  --hd-brand-deep: #0040a8;
  --hd-brand-bright: #4d8bff;
  --hd-brand-glow: rgba(0, 102, 255, 0.55);

  --hd-black: #000;
  --hd-ink: #1d1d1f;
  --hd-ink-soft: #424245;
  --hd-mute: #6e6e73;
  --hd-line: #d2d2d7;
  --hd-bg-soft: #f5f5f7;
  --hd-bg-white: #ffffff;

  --hd-nav-h: 52px;

  font-family:
    "SF Pro Display", "SF Pro Text", -apple-system, BlinkMacSystemFont,
    "Helvetica Neue", "PingFang SC", "Microsoft YaHei", "Noto Sans SC",
    sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  color: var(--hd-ink);
  background: #000;
  letter-spacing: -0.011em;
}
.home-shell :deep(*) { box-sizing: border-box; }

/* ============ 顶部固定导航 ============ */
.hd-nav {
  position: sticky; top: 0; z-index: 100;
  height: var(--hd-nav-h);
  background: rgba(0, 0, 0, 0.78);
  backdrop-filter: saturate(180%) blur(22px);
  -webkit-backdrop-filter: saturate(180%) blur(22px);
}
.hd-nav::after {
  content: ""; position: absolute; left: 0; right: 0; bottom: -1px;
  height: 1px;
  background: linear-gradient(90deg,
    transparent 0%,
    rgba(255, 255, 255, 0.08) 30%,
    rgba(255, 255, 255, 0.08) 70%,
    transparent 100%);
  pointer-events: none;
  opacity: 0.5;
}
.hd-nav-inner {
  max-width: 1280px; margin: 0 auto;
  padding: 0 22px;
  height: 100%;
  display: flex; align-items: center; justify-content: space-between;
  gap: 16px;
}
.hd-brand {
  display: inline-flex; align-items: center; gap: 9px;
  color: #fff;
  font-weight: 600; font-size: 14.5px;
  letter-spacing: -0.005em;
}
.hd-logo-wrap {
  width: 24px; height: 24px;
  display: grid; place-items: center;
  position: relative;
}
.hd-logo-wrap::after {
  content: ""; position: absolute; inset: -3px;
  border-radius: 8px;
  background: radial-gradient(circle, rgba(77, 139, 255, 0.4) 0%, transparent 70%);
  filter: blur(6px);
  pointer-events: none;
  z-index: -1;
}
.hd-logo-wrap--sm::after { display: none; }
.hd-logo-wrap--sm { width: 22px; height: 22px; }
.hd-logo-img { width: 100%; height: 100%; object-fit: contain; }
.hd-brand-name { color: rgba(255, 255, 255, 0.92); }
.hd-nav-right { display: flex; align-items: center; gap: 4px; }
.hd-icon-btn {
  display: inline-grid; place-items: center;
  width: 36px; height: 36px;
  border-radius: 999px;
  color: rgba(255,255,255,0.7);
  transition: background 0.15s ease, color 0.15s ease;
}
.hd-icon-btn:hover { background: rgba(255,255,255,0.08); color: #fff; }
.hd-cta-pill {
  display: inline-flex; align-items: center; gap: 8px;
  margin-left: 6px;
  height: 30px; padding: 0 14px 0 4px;
  border-radius: 999px;
  background: var(--hd-brand); color: #fff;
  font-size: 13px; font-weight: 600; letter-spacing: -0.01em;
  transition: background 0.18s ease;
}
.hd-cta-pill:not(:has(.hd-cta-avatar)) { padding: 0 16px; }
.hd-cta-pill:hover { background: #1f78ff; }
.hd-cta-avatar {
  width: 22px; height: 22px; border-radius: 50%;
  background: rgba(255,255,255,0.22);
  display: grid; place-items: center;
  font-size: 11px; font-weight: 700;
}

/* ============ 嵌入区 ============ */
.hd-embed-frame {
  height: calc(100vh - var(--hd-nav-h));
  width: 100%;
}
.hd-embed-html { min-height: calc(100vh - var(--hd-nav-h)); }

/* ============ Default Page ============ */
.hd-default { color: var(--hd-ink); }
.hd-section-inner {
  max-width: 1080px;
  margin: 0 auto;
  padding: 0 22px;
  position: relative;
  z-index: 1;
}

/* ============ HERO ============ */
.hd-hero {
  position: relative;
  background: #000;
  color: #fff;
  padding: 100px 0 60px;
  overflow: hidden;
  isolation: isolate;
}
.hd-hero-orb {
  position: absolute;
  width: 720px; height: 720px;
  border-radius: 50%;
  left: 50%; top: 280px;
  transform: translateX(-50%);
  background: radial-gradient(circle at 50% 50%,
    rgba(0, 102, 255, 0.55) 0%,
    rgba(0, 102, 255, 0.20) 30%,
    rgba(0, 0, 0, 0) 65%);
  filter: blur(20px);
  pointer-events: none;
  z-index: 0;
}
.hd-hero-orb--2 {
  width: 380px; height: 380px;
  left: 18%; top: 440px;
  background: radial-gradient(circle at 50% 50%,
    rgba(77, 139, 255, 0.35) 0%,
    rgba(77, 139, 255, 0.10) 35%,
    rgba(0, 0, 0, 0) 70%);
  filter: blur(30px);
  transform: none;
}
.hd-hero-grid-bg {
  position: absolute; inset: 0;
  background-image:
    linear-gradient(rgba(255,255,255,0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255,255,255,0.035) 1px, transparent 1px);
  background-size: 56px 56px;
  mask-image: linear-gradient(180deg,
    transparent 0%,
    rgba(0,0,0,0.4) 12%,
    rgba(0,0,0,0.8) 30%,
    rgba(0,0,0,0.4) 70%,
    transparent 100%);
  -webkit-mask-image: linear-gradient(180deg,
    transparent 0%,
    rgba(0,0,0,0.4) 12%,
    rgba(0,0,0,0.8) 30%,
    rgba(0,0,0,0.4) 70%,
    transparent 100%);
  pointer-events: none;
  z-index: 0;
}
.hd-hero-inner {
  position: relative; z-index: 1;
  max-width: 1080px; margin: 0 auto;
  padding: 0 22px;
  text-align: center;
}
.hd-eyebrow {
  display: inline-flex; align-items: center; gap: 10px;
  height: 32px; padding: 0 16px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: rgba(255, 255, 255, 0.85);
  font-size: 12.5px; font-weight: 500;
  letter-spacing: 0.05em;
  margin-bottom: 32px;
  backdrop-filter: blur(8px);
  animation: hd-fade-up 0.7s ease 0.05s both;
}
.hd-eyebrow-pulse {
  width: 7px; height: 7px; border-radius: 50%;
  background: #34d670;
  box-shadow: 0 0 0 4px rgba(52, 214, 112, 0.18);
  animation: hd-pulse 2.4s ease-in-out infinite;
}
.hd-hero-h1 {
  display: flex; flex-direction: column;
  font-size: clamp(48px, 7.5vw, 96px);
  font-weight: 700;
  letter-spacing: -0.03em;
  line-height: 1;
  margin: 0 0 28px;
}
.hd-hero-line {
  display: block;
  animation: hd-fade-up 0.8s ease 0.15s both;
}
.hd-hero-line + .hd-hero-line { animation-delay: 0.3s; }
.hd-hero-line--accent {
  background: linear-gradient(180deg, #fff 0%, #b9d3ff 60%, #4d8bff 100%);
  -webkit-background-clip: text; background-clip: text;
  -webkit-text-fill-color: transparent;
  color: transparent;
}
.hd-hero-sub {
  font-size: clamp(17px, 1.6vw, 22px);
  line-height: 1.45;
  font-weight: 400;
  color: rgba(255,255,255,0.7);
  max-width: 640px;
  margin: 0 auto 40px;
  animation: hd-fade-up 0.8s ease 0.45s both;
}
.hd-hero-actions {
  display: flex; gap: 24px;
  justify-content: center; align-items: center;
  flex-wrap: wrap;
  margin-bottom: 80px;
  animation: hd-fade-up 0.8s ease 0.6s both;
}

.hd-btn-primary {
  display: inline-flex; align-items: center;
  height: 48px; padding: 0 28px;
  border-radius: 999px;
  background: var(--hd-brand);
  color: #fff;
  font-size: 16px; font-weight: 500;
  letter-spacing: -0.01em;
  transition: background 0.18s ease, transform 0.18s ease, box-shadow 0.18s ease;
  box-shadow: 0 8px 24px rgba(0, 102, 255, 0.35);
}
.hd-btn-primary:hover { background: #1f78ff; transform: translateY(-1px); box-shadow: 0 12px 32px rgba(0, 102, 255, 0.45); }
.hd-btn-primary--lg { height: 56px; padding: 0 36px; font-size: 17px; }
.hd-btn-text {
  display: inline-flex; align-items: center; gap: 6px;
  color: var(--hd-brand-bright);
  font-size: 16px; font-weight: 500;
  letter-spacing: -0.01em;
}
.hd-btn-text .hd-arrow { transition: transform 0.18s ease; }
.hd-btn-text:hover .hd-arrow { transform: translateX(4px); }

.hd-hero-terminal-wrap {
  position: relative;
  max-width: 720px; margin: 0 auto;
  animation: hd-fade-up 0.9s ease 0.8s both;
}
.hd-terminal {
  background: linear-gradient(180deg, #1b1b1f 0%, #0e0e10 100%);
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow:
    0 30px 60px rgba(0, 0, 0, 0.45),
    0 0 0 1px rgba(0, 102, 255, 0.15),
    0 0 60px rgba(0, 102, 255, 0.2);
  overflow: hidden;
  text-align: left;
}
.hd-terminal-bar {
  display: flex; align-items: center; gap: 8px;
  padding: 14px 18px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(255, 255, 255, 0.02);
}
.hd-terminal-dot { width: 12px; height: 12px; border-radius: 50%; }
.hd-dot-r { background: #ff5f57; }
.hd-dot-y { background: #ffbd2e; }
.hd-dot-g { background: #28c940; }
.hd-terminal-title {
  flex: 1; text-align: center;
  font-size: 12px;
  font-family: "SF Mono", ui-monospace, monospace;
  color: rgba(255,255,255,0.4);
  margin-right: 56px;
}
.hd-terminal-body {
  padding: 22px 26px;
  font-family: "SF Mono", "JetBrains Mono", ui-monospace, monospace;
  font-size: 13.5px;
  line-height: 1.9;
}
.hd-code-line {
  display: flex; flex-wrap: wrap; gap: 8px;
  opacity: 0;
  animation: hd-fade-up 0.5s ease forwards;
}
.hd-line-1 { animation-delay: 1.0s; }
.hd-line-2 { animation-delay: 1.5s; }
.hd-line-3 { animation-delay: 2.0s; }
.hd-line-4 { animation-delay: 2.5s; }
.hd-line-5 { animation-delay: 3.1s; }
.hd-code-c1 { color: #34d670; font-weight: 600; }
.hd-code-c2 { color: #f4f4f5; }
.hd-code-c3 { color: var(--hd-brand-bright); }
.hd-code-c4 { color: #c5d8ff; }
.hd-code-c5 { color: #b794f4; }
.hd-code-c6 { color: #fbbf24; }
.hd-code-c7 { color: #6e6e73; font-style: italic; }
.hd-code-ok {
  color: #34d670; background: rgba(52, 214, 112, 0.12);
  padding: 1px 8px; border-radius: 4px; font-weight: 600;
}
.hd-code-resp { color: #f4f4f5; opacity: 0.85; }
.hd-cursor {
  display: inline-block;
  width: 8px; height: 16px;
  background: var(--hd-brand-bright);
  vertical-align: middle;
  animation: hd-blink 1.05s step-end infinite;
}

/* ============ 关键数字 ============ */
.hd-numbers {
  background: #000;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  padding: 60px 0;
}
.hd-num-grid {
  display: grid; grid-template-columns: repeat(4, 1fr);
  gap: 32px;
  text-align: center;
}
.hd-num-v {
  font-size: clamp(40px, 4.5vw, 64px);
  font-weight: 600; letter-spacing: -0.04em; line-height: 1;
  color: #fff;
}
.hd-num-v small {
  font-size: 0.45em;
  color: var(--hd-brand-bright);
  margin-left: 2px;
  font-weight: 600;
}
.hd-num-k {
  font-size: 14px;
  color: rgba(255,255,255,0.55);
  margin-top: 14px;
  letter-spacing: 0;
}

/* ============ Spotlight ============ */
.hd-spotlight {
  padding: 140px 0;
  position: relative;
  overflow: hidden;
}
.hd-section--tight { padding: 100px 0; }
.hd-spotlight--light { background: var(--hd-bg-soft); color: var(--hd-ink); }
.hd-spotlight--dark {
  background: #000; color: #fff;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}
.hd-spot-eyebrow {
  font-size: 13px; font-weight: 700;
  color: var(--hd-brand);
  letter-spacing: 0.18em;
  margin-bottom: 18px;
  text-transform: uppercase;
}
.hd-spot-eyebrow--dark { color: var(--hd-brand-bright); }
.hd-spot-title {
  font-size: clamp(40px, 5vw, 72px);
  font-weight: 600;
  letter-spacing: -0.03em;
  line-height: 1.05;
  margin: 0 0 24px;
  color: var(--hd-ink);
}
.hd-spot-title--dark { color: #fff; }
.hd-text-grad-blue {
  background: linear-gradient(135deg, #0066ff 0%, #4d8bff 100%);
  -webkit-background-clip: text; background-clip: text;
  -webkit-text-fill-color: transparent;
  color: transparent;
}
.hd-text-grad-blue-bright {
  background: linear-gradient(135deg, #4d8bff 0%, #b9d3ff 100%);
  -webkit-background-clip: text; background-clip: text;
  -webkit-text-fill-color: transparent;
  color: transparent;
}
.hd-spot-sub {
  font-size: clamp(17px, 1.5vw, 21px);
  line-height: 1.5;
  color: var(--hd-mute);
  max-width: 720px;
  margin-bottom: 64px;
  font-weight: 400;
}
.hd-spot-sub--dark { color: rgba(255,255,255,0.65); }

/* ============ Provider 舞台（Spotlight 1）============ */
.hd-providers-stage {
  position: relative;
  height: 360px;
  margin-top: 40px;
  display: grid;
  place-items: center;
}
.hd-prov-card {
  position: absolute;
  display: flex; align-items: center; gap: 10px;
  padding: 12px 18px 12px 12px;
  background: #fff;
  border: 1px solid var(--hd-line);
  border-radius: 14px;
  box-shadow: 0 12px 28px rgba(0,0,0,0.06);
  transition: transform 0.4s cubic-bezier(0.2, 0.8, 0.2, 1);
}
.hd-prov-card:hover { transform: translateY(-4px) scale(1.04); }
.hd-prov-mark {
  width: 32px; height: 32px;
  border-radius: 9px;
  color: #fff; font-weight: 700; font-size: 14px;
  display: grid; place-items: center;
}
.hd-prov-label { font-size: 14.5px; font-weight: 600; color: var(--hd-ink); }
.hd-prov-claude { left: 6%; top: 14%; }
.hd-prov-claude .hd-prov-mark { background: linear-gradient(135deg, #d97757, #b85a3e); }
.hd-prov-gpt { right: 6%; top: 14%; }
.hd-prov-gpt .hd-prov-mark { background: linear-gradient(135deg, #10a37f, #0d8a6a); }
.hd-prov-gemini { left: 2%; bottom: 14%; }
.hd-prov-gemini .hd-prov-mark { background: linear-gradient(135deg, #4285f4, #1f5dc9); }
.hd-prov-anti { right: 2%; bottom: 14%; }
.hd-prov-anti .hd-prov-mark { background: linear-gradient(135deg, #f43f5e, #be185d); }
.hd-prov-more {
  left: 50%; bottom: -8%;
  transform: translateX(-50%);
  background: rgba(255,255,255,0.7);
  border-style: dashed;
}
.hd-prov-more .hd-prov-mark { background: linear-gradient(135deg, #64748b, #475569); }
.hd-prov-more:hover { transform: translateX(-50%) translateY(-4px) scale(1.04); }

/* 中心枢纽 */
.hd-prov-hub {
  position: relative;
  width: 130px; height: 130px;
  display: grid; place-items: center;
}
.hd-prov-hub-ring {
  position: absolute; inset: 0;
  border-radius: 50%;
  border: 1.5px solid var(--hd-brand);
  background: radial-gradient(circle at 50% 50%, rgba(0,102,255,0.06) 0%, transparent 70%);
  box-shadow:
    0 0 0 12px rgba(0, 102, 255, 0.06),
    0 0 0 28px rgba(0, 102, 255, 0.03);
  animation: hd-pulse-ring 3s ease-in-out infinite;
}
.hd-prov-hub-core {
  position: relative; z-index: 1;
  width: 68px; height: 68px;
  border-radius: 16px;
  background: #fff;
  box-shadow:
    0 14px 36px rgba(0, 102, 255, 0.28),
    0 0 0 1px rgba(0, 102, 255, 0.15);
  display: grid; place-items: center;
  overflow: hidden;
}
.hd-prov-hub-core img { width: 80%; height: 80%; object-fit: contain; }

/* ============ Bento（Spotlight 2）============ */
.hd-bento {
  display: grid;
  grid-template-columns: 1.4fr 1fr;
  grid-template-rows: 1fr 1fr;
  gap: 18px;
  margin-top: 32px;
}
.hd-bento-tile {
  position: relative;
  background: linear-gradient(180deg, rgba(255,255,255,0.06) 0%, rgba(255,255,255,0.02) 100%);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 22px;
  padding: 36px 36px 0;
  overflow: hidden;
  transition: border-color 0.2s ease, transform 0.3s ease;
  isolation: isolate;
  min-height: 280px;
}
.hd-bento-tile:hover {
  border-color: rgba(77, 139, 255, 0.35);
  transform: translateY(-3px);
}
.hd-bento-num {
  position: absolute; top: 24px; right: 28px;
  font-family: "SF Mono", ui-monospace, monospace;
  font-size: 12px;
  color: var(--hd-brand-bright);
  letter-spacing: 0.1em;
  opacity: 0.85;
}
.hd-bento-tile h3 {
  font-size: 26px; font-weight: 600;
  letter-spacing: -0.02em;
  margin: 0 0 12px;
  color: #fff;
}
.hd-bento-tile p {
  font-size: 15px; line-height: 1.55;
  color: rgba(255,255,255,0.62);
  max-width: 360px;
}
.hd-bento-1 { grid-row: 1; grid-column: 1; }
.hd-bento-2 { grid-row: 1; grid-column: 2; }
.hd-bento-3 { grid-row: 2; grid-column: 1; }
.hd-bento-4 { grid-row: 2; grid-column: 2; }

/* Bento visuals */
.hd-bento-viz {
  position: absolute;
  pointer-events: none;
}
.hd-viz-session {
  right: -40px; bottom: -40px;
  width: 280px; height: 220px;
  background:
    radial-gradient(circle at 30% 50%, rgba(0,102,255,0.4), transparent 50%),
    radial-gradient(circle at 70% 60%, rgba(77,139,255,0.3), transparent 50%);
  filter: blur(10px);
}
.hd-viz-pool {
  right: -10px; bottom: -10px;
  width: 200px; height: 160px;
  background-image:
    linear-gradient(90deg, rgba(0,102,255,0.4) 1px, transparent 1px),
    linear-gradient(rgba(0,102,255,0.4) 1px, transparent 1px);
  background-size: 24px 24px;
  mask-image: radial-gradient(circle at 80% 100%, rgba(0,0,0,1), transparent 70%);
  -webkit-mask-image: radial-gradient(circle at 80% 100%, rgba(0,0,0,1), transparent 70%);
}
.hd-viz-billing {
  right: -10px; bottom: 0;
  width: 320px; height: 140px;
  background:
    linear-gradient(180deg, transparent 60%, rgba(0,102,255,0.25) 100%);
  -webkit-mask-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 320 140' preserveAspectRatio='none'%3E%3Cpath d='M0 100 Q40 80 80 90 T160 70 T240 50 T320 30 V140 H0 Z' fill='black'/%3E%3C/svg%3E");
  mask-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 320 140' preserveAspectRatio='none'%3E%3Cpath d='M0 100 Q40 80 80 90 T160 70 T240 50 T320 30 V140 H0 Z' fill='black'/%3E%3C/svg%3E");
  -webkit-mask-size: 100% 100%;
  mask-size: 100% 100%;
}
.hd-viz-obs {
  right: -20px; bottom: -20px;
  width: 220px; height: 180px;
  background:
    radial-gradient(circle at 50% 50%, rgba(77,139,255,0.4) 0%, transparent 50%);
  border: 1px dashed rgba(77,139,255,0.4);
  border-radius: 50%;
  filter: blur(2px);
}

/* ============ Compare ============ */
.hd-compare {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
  margin-top: 56px;
}
.hd-cmp-col {
  background: #fff;
  border: 1px solid var(--hd-line);
  border-radius: 22px;
  padding: 36px 36px 32px;
  transition: transform 0.3s ease, box-shadow 0.3s ease, border-color 0.3s ease;
}
.hd-cmp-col:hover { transform: translateY(-3px); }
.hd-cmp-col--us {
  background: linear-gradient(180deg, #001844 0%, #000814 100%);
  border-color: rgba(77,139,255,0.4);
  color: #fff;
  box-shadow: 0 30px 60px rgba(0,102,255,0.18);
  position: relative;
}
.hd-cmp-col--us::before {
  content: ""; position: absolute; inset: -1px;
  border-radius: 22px;
  padding: 1px;
  background: linear-gradient(135deg, rgba(77,139,255,0.5), transparent 60%);
  -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
  -webkit-mask-composite: xor; mask-composite: exclude;
  pointer-events: none;
}
.hd-cmp-head { margin-bottom: 28px; }
.hd-cmp-tag {
  display: inline-flex;
  height: 26px; padding: 0 12px;
  border-radius: 999px;
  font-size: 12px; font-weight: 600;
  letter-spacing: 0.04em;
  align-items: center;
  margin-bottom: 14px;
}
.hd-cmp-tag--off { background: rgba(0,0,0,0.05); color: var(--hd-mute); }
.hd-cmp-tag--us { background: var(--hd-brand); color: #fff; }
.hd-cmp-headline {
  font-size: 22px; font-weight: 600;
  letter-spacing: -0.02em;
}
.hd-cmp-col--us .hd-cmp-headline { color: #fff; }
.hd-cmp-col ul { list-style: none; padding: 0; margin: 0; }
.hd-cmp-col li {
  display: flex; gap: 14px;
  padding: 14px 0;
  border-top: 1px solid var(--hd-line);
  align-items: flex-start;
}
.hd-cmp-col--us li { border-top-color: rgba(255,255,255,0.08); }
.hd-cmp-cross, .hd-cmp-check {
  flex-shrink: 0;
  width: 22px; height: 22px;
  border-radius: 50%;
  display: grid; place-items: center;
  font-size: 13px; font-weight: 700;
  margin-top: 1px;
}
.hd-cmp-cross { background: rgba(0,0,0,0.05); color: var(--hd-mute); }
.hd-cmp-check { background: var(--hd-brand); color: #fff; box-shadow: 0 0 0 4px rgba(0,102,255,0.15); }
.hd-cmp-feat { font-size: 13px; color: var(--hd-mute); margin-bottom: 2px; }
.hd-cmp-col--us .hd-cmp-feat { color: rgba(255,255,255,0.5); }
.hd-cmp-val { font-size: 15px; font-weight: 500; color: var(--hd-ink); line-height: 1.5; letter-spacing: -0.01em; }
.hd-cmp-col--us .hd-cmp-val { color: #fff; }

/* ============ Final CTA ============ */
.hd-final-cta {
  position: relative;
  background: #000;
  color: #fff;
  padding: 140px 0;
  overflow: hidden;
  text-align: center;
}
.hd-final-orb {
  position: absolute; inset: 0;
  background:
    radial-gradient(circle at 50% 100%, rgba(0,102,255,0.45) 0%, transparent 50%),
    radial-gradient(circle at 50% 0%, rgba(0,102,255,0.15) 0%, transparent 50%);
  filter: blur(40px);
  pointer-events: none;
}
.hd-final-inner { position: relative; z-index: 1; }
.hd-final-title {
  font-size: clamp(40px, 5.5vw, 80px);
  font-weight: 600;
  letter-spacing: -0.03em;
  line-height: 1.05;
  margin: 0 0 22px;
}
.hd-final-sub {
  font-size: clamp(17px, 1.5vw, 21px);
  color: rgba(255,255,255,0.65);
  max-width: 600px;
  margin: 0 auto 44px;
  line-height: 1.5;
}

/* ============ Footer ============ */
.hd-footer {
  background: var(--hd-bg-soft);
  border-top: 1px solid var(--hd-line);
  padding: 32px 0;
}
.hd-footer-inner {
  display: flex; align-items: center; justify-content: space-between;
  gap: 16px; flex-wrap: wrap;
}
.hd-footer-brand {
  display: inline-flex; align-items: center; gap: 8px;
  font-size: 13px; font-weight: 500;
  color: var(--hd-ink);
}
.hd-footer-meta {
  display: inline-flex; align-items: center; gap: 24px;
  font-size: 12.5px;
  color: var(--hd-mute);
}
.hd-footer-meta a { color: var(--hd-mute); transition: color 0.15s ease; }
.hd-footer-meta a:hover { color: var(--hd-brand); }

/* ============ 动画 ============ */
@keyframes hd-fade-up {
  from { opacity: 0; transform: translateY(16px); }
  to { opacity: 1; transform: translateY(0); }
}
@keyframes hd-blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}
@keyframes hd-pulse {
  0%, 100% { box-shadow: 0 0 0 4px rgba(52, 214, 112, 0.18); }
  50% { box-shadow: 0 0 0 7px rgba(52, 214, 112, 0.10); }
}
@keyframes hd-pulse-ring {
  0%, 100% {
    box-shadow:
      0 0 0 12px rgba(0, 102, 255, 0.08),
      0 0 0 28px rgba(0, 102, 255, 0.04);
  }
  50% {
    box-shadow:
      0 0 0 18px rgba(0, 102, 255, 0.12),
      0 0 0 40px rgba(0, 102, 255, 0.04);
  }
}

/* ============ 响应式 ============ */
@media (max-width: 900px) {
  .hd-hero { padding: 72px 0 40px; }
  .hd-spotlight { padding: 100px 0; }
  .hd-num-grid { grid-template-columns: repeat(2, 1fr); gap: 40px 20px; }
  .hd-bento { grid-template-columns: 1fr; }
  .hd-bento-1, .hd-bento-2, .hd-bento-3, .hd-bento-4 { grid-column: 1; grid-row: auto; }
  .hd-compare { grid-template-columns: 1fr; }
  .hd-providers-stage { height: 480px; }
  .hd-prov-card { transform: scale(0.92); }
  .hd-prov-claude { left: 4%; top: 6%; }
  .hd-prov-gpt { right: 4%; top: 6%; }
  .hd-prov-gemini { left: 4%; bottom: 22%; }
  .hd-prov-anti { right: 4%; bottom: 22%; }
}
@media (max-width: 640px) {
  .hd-hero-actions { gap: 16px; }
  .hd-hero-terminal-wrap { padding: 0 4px; }
  .hd-terminal-body { padding: 16px 18px; font-size: 12px; }
  .hd-spotlight { padding: 80px 0; }
  .hd-bento-tile { padding: 28px 24px 0; min-height: 240px; }
  .hd-cmp-col { padding: 28px 24px 24px; }
  .hd-final-cta { padding: 100px 0; }
  .hd-nav-inner { padding: 0 16px; }
  .hd-brand-name { display: none; }
}
</style>
