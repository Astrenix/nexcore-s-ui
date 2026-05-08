<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.client')"
    destroy-on-close
  >
    <div v-if="loading" class="modal-loading">
      <el-icon class="is-loading"><Loading /></el-icon>
    </div>

    <el-tabs v-else v-model="tab" class="client-tabs">
      <el-tab-pane :label="$t('client.basics')" name="t1">
        <el-form label-position="top">
          <div class="form-grid">
            <el-form-item :label="$t('enable')">
              <el-switch v-model="client.enable" />
            </el-form-item>
            <el-form-item :label="$t('client.group')">
              <el-select v-model="client.group" allow-create filterable default-first-option>
                <el-option v-for="g in groups" :key="g" :label="g.length > 0 ? g : $t('none')" :value="g" />
              </el-select>
            </el-form-item>
            <el-form-item :label="$t('client.name')">
              <el-input v-model="client.name" />
            </el-form-item>
            <el-form-item :label="$t('client.desc')">
              <el-input v-model="client.desc" />
            </el-form-item>
            <el-form-item :label="`${$t('stats.volume')} (GiB)`">
              <el-input-number v-model="Volume" :min="0" controls-position="right" style="width: 100%" />
            </el-form-item>
            <DatePick v-if="!(client.delayStart && !client.autoReset)" :expiry="expDate" @submit="setDate" />
            <el-form-item v-if="client.autoReset || client.delayStart" :label="$t('client.resetDays')">
              <el-input-number v-model="resetDays" :min="1" controls-position="right" style="width: 100%" />
            </el-form-item>
            <el-form-item :label="$t('client.delayStart')">
              <el-switch v-model="delayStart" :disabled="client.up + client.down > 0" />
            </el-form-item>
            <el-form-item :label="$t('client.autoReset')">
              <el-switch v-model="autoReset" />
            </el-form-item>
          </div>

          <div v-if="id > 0" class="usage-summary">
            <div class="usage-summary__row">
              <span>{{ $t('stats.usage') }}: <span class="mono">{{ total }}</span><sup v-if="percent > 0" class="mono">({{ percent }}%)</sup></span>
              <el-tooltip :content="$t('reset')" placement="top">
                <el-button text @click="resetUsage"><el-icon><RefreshRight /></el-icon></el-button>
              </el-tooltip>
            </div>
            <el-progress
              v-if="client.volume > 0"
              :percentage="percent"
              :status="percentStatus"
              :stroke-width="4"
              :show-text="false"
            />
            <div class="usage-summary__row mono">
              <span><el-icon style="color: var(--nc-warning)"><Upload /></el-icon> {{ up }}</span>
              <span>/</span>
              <span><el-icon style="color: var(--nc-success)"><Download /></el-icon> {{ down }}</span>
            </div>
            <div v-if="client.autoReset" class="usage-summary__reset">
              <div>{{ $t('client.nextReset') }}: <span class="mono">{{ nextResetFormatted }}</span></div>
              <div class="mono">↑ {{ totalUp }} / ↓ {{ totalDown }}</div>
            </div>
          </div>

          <el-form-item :label="$t('client.inboundTags')">
            <el-select v-model="clientInbounds" multiple collapse-tags collapse-tags-tooltip filterable>
              <el-option v-for="t in inboundTags" :key="t.value" :label="t.title" :value="t.value" />
            </el-select>
            <el-button text style="margin-left: 4px" @click="setAllInbounds">{{ $t('all') }}</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <el-tab-pane :label="$t('client.config')" name="t2">
        <div class="config-section">
          <el-button @click="shuffle()">
            <el-icon><RefreshRight /></el-icon>{{ $t('reset') }} - {{ $t('all') }}
          </el-button>
        </div>
        <el-form label-position="top">
          <div v-for="key in Object.keys(clientConfig)" :key="key" class="config-row">
            <div class="config-row__head">
              <span class="config-row__name">{{ key }}</span>
              <el-tooltip :content="$t('reset')" placement="top">
                <el-button text @click="shuffle(key)"><el-icon><RefreshRight /></el-icon></el-button>
              </el-tooltip>
            </div>
            <el-input v-if="clientConfig[key].password != undefined" v-model="clientConfig[key].password" placeholder="Password" />
            <el-input v-if="clientConfig[key].uuid != undefined" v-model="clientConfig[key].uuid" placeholder="UUID" />
            <el-input v-if="key == 'vless'" v-model="clientConfig[key].flow" placeholder="Flow" />
            <el-input v-if="key == 'hysteria'" v-model="clientConfig[key].auth_str" placeholder="Auth" />
          </div>
        </el-form>
      </el-tab-pane>

      <el-tab-pane :label="$t('client.links')" name="t3">
        <div class="links-list">
          <div v-for="(lnk, i) in links" :key="`local-${i}`" class="link-row">
            <span class="link-row__index">{{ i + 1 }}</span>
            <code class="link-row__uri">{{ lnk.uri }}</code>
          </div>
        </div>

        <div class="links-section">
          <el-button @click="extLinks.push({ type: 'external', uri: '' })">
            <el-icon><Plus /></el-icon>{{ $t('actions.add') }} {{ $t('client.external') }}
          </el-button>
          <div v-for="(lnk, i) in extLinks" :key="`ext-${i}`" class="link-input-row">
            <el-input v-model="lnk.uri" :placeholder="`<protocol>://<data>`" dir="ltr">
              <template #prepend>{{ $t('client.external') }} {{ i + 1 }}</template>
              <template #append>
                <el-button @click="extLinks.splice(i, 1)"><el-icon><Delete /></el-icon></el-button>
              </template>
            </el-input>
          </div>
        </div>

        <div class="links-section">
          <el-button @click="subLinks.push({ type: 'sub', uri: '' })">
            <el-icon><Plus /></el-icon>{{ $t('actions.add') }} {{ $t('client.sub') }}
          </el-button>
          <div v-for="(lnk, i) in subLinks" :key="`sub-${i}`" class="link-input-row">
            <el-input v-model="lnk.uri" placeholder="http[s]://<domain>[:]<port>/<path>" dir="ltr">
              <template #prepend>{{ $t('client.sub') }} {{ i + 1 }}</template>
              <template #append>
                <el-button @click="subLinks.splice(i, 1)"><el-icon><Delete /></el-icon></el-button>
              </template>
            </el-input>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :loading="loading" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch, computed } from 'vue'
import { createClient, randomConfigs, updateConfigs, Link, shuffleConfigs } from '@/types/clients'
import DatePick from '@/components/DateTime.vue'
import { HumanReadable } from '@/plugins/utils'
import Data from '@/store/modules/data'
import { locale } from '@/locales'
import { Loading, Upload, Download, RefreshRight, Plus, Delete } from '@element-plus/icons-vue'

const props = defineProps<{ visible: boolean; id: number; inboundTags: any[]; groups: string[] }>()
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const client = ref<any>(createClient())
const title = ref('add')
const loading = ref(false)
const tab = ref('t1')
const clientConfig = ref<any>({})
const links = ref<Link[]>([])
const extLinks = ref<Link[]>([])
const subLinks = ref<Link[]>([])

const updateData = async (id: number) => {
  if (id > 0) {
    loading.value = true
    const newData = await Data().loadClients(id)
    client.value = createClient(newData)
    title.value = 'edit'
    clientConfig.value = client.value.config
    loading.value = false
  } else {
    client.value = createClient()
    title.value = 'add'
    clientConfig.value = randomConfigs('client')
  }
  links.value = client.value.links?.filter((l: Link) => l.type === 'local') ?? []
  extLinks.value = client.value.links?.filter((l: Link) => l.type === 'external') ?? []
  subLinks.value = client.value.links?.filter((l: Link) => l.type === 'sub') ?? []
  tab.value = 't1'
  loading.value = false
}

const closeModal = () => {
  updateData(0)
  emit('close')
}

const saveChanges = async () => {
  if (!props.visible) return
  if (Data().checkClientName(props.id, client.value.name)) return
  if (client.value.delayStart && !client.value.autoReset) client.value.expiry = 0
  loading.value = true
  client.value.config = updateConfigs(clientConfig.value, client.value.name)
  client.value.links = [
    ...extLinks.value.filter((l) => l.uri !== ''),
    ...subLinks.value.filter((l) => l.uri !== ''),
  ]
  const success = await Data().save('clients', props.id == 0 ? 'new' : 'edit', client.value)
  if (success) closeModal()
  loading.value = false
}

const setDate = (v: number) => { client.value.expiry = v }

const setAllInbounds = () => {
  client.value.inbounds = props.inboundTags.map((i: any) => i.value).sort()
}

const shuffle = (k?: string) => shuffleConfigs(clientConfig.value, k)

const resetUsage = () => {
  client.value.totalUp = (client.value.totalUp ?? 0) + client.value.up
  client.value.totalDown = (client.value.totalDown ?? 0) + client.value.down
  client.value.up = 0
  client.value.down = 0
}

const clientInbounds = computed({
  get: () => (client.value.inbounds?.length > 0 ? [...client.value.inbounds].sort() : []),
  set: (v: number[]) => { client.value.inbounds = v.length === 0 ? [] : [...v].sort() },
})
const expDate = computed(() => client.value.expiry)
const Volume = computed({
  get: () => (client.value.volume === 0 ? 0 : client.value.volume / 1024 ** 3),
  set: (v: number) => { client.value.volume = v > 0 ? v * 1024 ** 3 : 0 },
})
const delayStart = computed({
  get: () => client.value.delayStart ?? false,
  set: (v: boolean) => {
    client.value.delayStart = v
    client.value.resetDays = v ? 1 : 0
    if (v && !autoReset.value) client.value.expiry = 0
  },
})
const autoReset = computed({
  get: () => client.value.autoReset ?? false,
  set: (v: boolean) => {
    client.value.autoReset = v
    client.value.resetDays = v ? 1 : 0
    if (!v) client.value.nextReset = 0
  },
})
const resetDays = computed({
  get: () => client.value.resetDays ?? 1,
  set: (v: number | null) => {
    if (!v) v = 1
    if (client.value.nextReset && client.value.nextReset > 0) {
      client.value.nextReset += (v - (client.value.resetDays ?? 0)) * 24 * 60 * 60
    }
    client.value.resetDays = v
  },
})
const up = computed(() => HumanReadable.sizeFormat(client.value.up))
const down = computed(() => HumanReadable.sizeFormat(client.value.down))
const total = computed(() => HumanReadable.sizeFormat(client.value.up + client.value.down))
const totalUp = computed(() => HumanReadable.sizeFormat((client.value.totalUp ?? 0) + client.value.up))
const totalDown = computed(() => HumanReadable.sizeFormat((client.value.totalDown ?? 0) + client.value.down))
const nextResetFormatted = computed(() => {
  const ts = client.value.nextReset ?? 0
  if (ts === 0) return '—'
  return new Date(ts * 1000).toLocaleString(locale)
})
const percent = computed(() => (client.value.volume > 0 ? Math.round(((client.value.up + client.value.down) * 100) / client.value.volume) : 0))
const percentStatus = computed<'success' | 'warning' | 'exception'>(() =>
  client.value.up + client.value.down >= client.value.volume ? 'exception' : percent.value > 90 ? 'warning' : 'success',
)

watch(() => props.visible, (v) => { if (v) updateData(props.id) })
</script>

<style scoped>
.modal-loading {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 64px 0;
  font-size: 32px;
  color: var(--nc-primary);
}
.client-tabs {
  background: transparent;
}
.client-tabs :deep(.el-tabs__nav-wrap::after) { display: none; }

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 6px 16px;
}

.usage-summary {
  background: var(--nc-border-soft);
  border-radius: var(--radius-md);
  padding: 12px 14px;
  margin: 6px 0 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12.5px;
}

.usage-summary__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.usage-summary__reset {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px;
  font-size: 11.5px;
  color: var(--nc-text-muted);
}

.config-section {
  margin-bottom: 12px;
}

.config-row {
  display: grid;
  grid-template-columns: 110px 1fr;
  gap: 10px;
  margin-bottom: 10px;
  align-items: center;
}

.config-row__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  font-size: 12px;
  font-weight: 600;
  color: var(--nc-text-muted);
}

.config-row__name {
  font-family: var(--font-display);
}

.links-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 14px;
}

.link-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  background: var(--nc-border-soft);
  padding: 6px 10px;
  border-radius: var(--radius-md);
}

.link-row__index {
  font-family: var(--font-mono);
  color: var(--nc-text-muted);
  flex-shrink: 0;
}

.link-row__uri {
  font-family: var(--font-mono);
  color: var(--nc-text-3);
  word-break: break-all;
}

.links-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 14px;
}

.link-input-row {
  display: flex;
  gap: 4px;
}
</style>
