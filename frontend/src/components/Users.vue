<template>
  <div class="nc-card">
    <h4 class="nc-card-title">{{ $t('pages.clients') }}</h4>
    <div class="users-row">
      <el-form-item :label="$t('actions.action')" class="users-row__item">
        <el-select v-model="data.model" @change="data.values = []">
          <el-option v-for="m in initUsersModels" :key="m.value" :label="m.title" :value="m.value" />
        </el-select>
      </el-form-item>
      <el-form-item v-if="data.model === 'group'" :label="$t('client.group')" class="users-row__item users-row__item--wide">
        <el-select v-model="data.values" multiple collapse-tags collapse-tags-tooltip>
          <el-option v-for="g in groupNames" :key="g" :label="g.length > 0 ? g : $t('none')" :value="g" />
        </el-select>
      </el-form-item>
      <el-form-item v-if="data.model === 'client'" :label="$t('pages.clients')" class="users-row__item users-row__item--wide">
        <el-select v-model="data.values" multiple collapse-tags collapse-tags-tooltip filterable>
          <el-option v-for="c in clientNames" :key="c.value" :label="c.title" :value="c.value" />
        </el-select>
      </el-form-item>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import { i18n } from '@/locales'

const props = defineProps<{ data: { model: string; values: any[] }; clients: any[] }>()

const initUsersModels = [
  { title: i18n.global.t('none'), value: 'none' },
  { title: i18n.global.t('all'), value: 'all' },
  { title: i18n.global.t('client.group'), value: 'group' },
  { title: i18n.global.t('pages.clients'), value: 'client' },
]

const clientNames = computed(() => (props.clients ?? []).map((c: any) => ({ title: c.name, value: c.id })))
const groupNames = computed(() => Array.from(new Set((props.clients ?? []).map((c: any) => c.group))))

defineExpose({ data: props.data })
</script>

<style scoped>
.nc-card-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--nc-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 10px;
}

.users-row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 16px;
}

.users-row__item {
  flex: 0 0 220px;
  margin: 0 !important;
}

.users-row__item--wide {
  flex: 1 1 320px;
  min-width: 240px;
}

.users-row :deep(.el-form-item__label) {
  font-size: 12px;
  color: var(--nc-text-muted);
}
</style>
