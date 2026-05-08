<template>
  <el-form-item :label="$t('date.expiry')">
    <div class="datetime-row">
      <el-date-picker
        v-model="picked"
        type="datetime"
        :placeholder="$t('unlimited')"
        format="YYYY-MM-DD HH:mm"
        value-format="x"
        :clearable="true"
        style="flex: 1"
        @change="onChange"
      />
      <el-tag v-if="!picked" type="success" size="small" effect="plain">{{ $t('unlimited') }}</el-tag>
    </div>
  </el-form-item>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue'

const props = defineProps<{ expiry: number | string | undefined }>()
const emit = defineEmits<{ submit: [v: number] }>()

const expSec = computed(() => parseInt((props.expiry ?? 0) as any) || 0)

const picked = ref<string | null>(expSec.value > 0 ? String(expSec.value * 1000) : null)

watch(() => props.expiry, () => {
  picked.value = expSec.value > 0 ? String(expSec.value * 1000) : null
})

const onChange = (v: string | null) => {
  if (!v) {
    emit('submit', 0)
    return
  }
  const ms = parseInt(v)
  emit('submit', Math.floor(ms / 1000))
}
</script>

<style scoped>
.datetime-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
