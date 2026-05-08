import { computed, onMounted, onUnmounted, ref } from 'vue'

/**
 * 响应式断点(替代 Vuetify 的 useDisplay)
 *
 * 断点对齐 NexCore design-system.md:
 *   xs:  < 576px
 *   sm:  >= 576px
 *   md:  >= 768px
 *   lg:  >= 992px
 *   xl:  >= 1200px
 *   xxl: >= 1440px
 */

export type Breakpoint = 'xs' | 'sm' | 'md' | 'lg' | 'xl' | 'xxl'

const BP_MAP: Record<Exclude<Breakpoint, 'xs'>, number> = {
  sm: 576,
  md: 768,
  lg: 992,
  xl: 1200,
  xxl: 1440,
}

export function useBreakpoint() {
  const width = ref(typeof window !== 'undefined' ? window.innerWidth : 1280)

  const update = () => { width.value = window.innerWidth }

  onMounted(() => window.addEventListener('resize', update, { passive: true }))
  onUnmounted(() => window.removeEventListener('resize', update))

  const current = computed<Breakpoint>(() => {
    const w = width.value
    if (w >= BP_MAP.xxl) return 'xxl'
    if (w >= BP_MAP.xl) return 'xl'
    if (w >= BP_MAP.lg) return 'lg'
    if (w >= BP_MAP.md) return 'md'
    if (w >= BP_MAP.sm) return 'sm'
    return 'xs'
  })

  return {
    width,
    current,
    xs: computed(() => width.value < BP_MAP.sm),
    smAndDown: computed(() => width.value < BP_MAP.md),
    mdAndDown: computed(() => width.value < BP_MAP.lg),
    lgAndDown: computed(() => width.value < BP_MAP.xl),
    smAndUp: computed(() => width.value >= BP_MAP.sm),
    mdAndUp: computed(() => width.value >= BP_MAP.md),
    lgAndUp: computed(() => width.value >= BP_MAP.lg),
    xlAndUp: computed(() => width.value >= BP_MAP.xl),
    isMobile: computed(() => width.value < BP_MAP.md),
  }
}
