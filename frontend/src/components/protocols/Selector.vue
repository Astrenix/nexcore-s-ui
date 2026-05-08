<template>
  <div class="proto-stub">
    <span class="proto-stub__name">Selector</span>
    <span class="proto-stub__hint">完整字段编辑请使用上层 JSON 编辑器(阶段 4 stub,功能未受影响,通过 JSON 编辑配置)</span>
  </div>
</template>

<script lang="ts" setup>
defineProps<{
  data?: any
  direction?: string
  inbound?: any
  outbound?: any
  inTags?: any
  tlsConfigs?: any
  tls_id?: number
  tags?: any
  clients?: any
  addr?: any
  hasTls?: boolean
  type?: string
  inData?: any
  dial?: any
  mode?: string
  peers?: any
  tilesData?: any
}>()
</script>

<style scoped>
.proto-stub {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 14px;
  background: #f8fafc;
  border: 1px dashed var(--nc-border);
  border-radius: var(--radius-md);
  margin: 8px 0;
}
.proto-stub__name {
  font-size: 11.5px;
  font-weight: 600;
  color: var(--nc-primary);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.proto-stub__hint {
  font-size: 11.5px;
  color: var(--nc-text-muted);
}
</style>
