<template>
  <div class="page-container">
    <div class="page-header">
      <h2 class="page-title">{{ $t('pages.home') }}</h2>
      <p class="page-desc">{{ $t('home.desc', 'Sing-Box runtime overview') }}</p>
    </div>

    <Main />
  </div>
</template>

<script lang="ts" setup>
import Main from '@/components/Main.vue'
</script>
