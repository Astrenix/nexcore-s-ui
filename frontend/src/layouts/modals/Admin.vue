<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog"
    :align-center="false"
    :title="`${$t('admin.changeCred')} ${user?.username || ''}`"
    destroy-on-close
  >
    <el-form ref="formRef" :model="newData" :rules="rules" label-position="top">
      <el-form-item :label="$t('admin.oldPass')" prop="oldPass">
        <el-input v-model="newData.oldPass" type="password" show-password />
      </el-form-item>
      <el-form-item :label="$t('admin.newUname')" prop="newUsername">
        <el-input v-model="newData.newUsername" />
      </el-form-item>
      <el-form-item :label="$t('admin.newPass')" prop="newPass">
        <el-input v-model="newData.newPass" type="password" show-password />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { reactive, ref, watch } from 'vue'
import { i18n } from '@/locales'
import type { FormInstance, FormRules } from 'element-plus'

const props = defineProps<{ visible: boolean; user: any }>()
const emit = defineEmits<{ close: []; save: [data: any]; 'update:modelValue': [v: boolean] }>()

const formRef = ref<FormInstance>()
const newData = reactive({ id: 0, oldPass: '', newUsername: '', newPass: '' })

const rules: FormRules = {
  oldPass:     [{ required: true, message: i18n.global.t('login.pwRules'), trigger: 'blur' }],
  newUsername: [{ required: true, message: i18n.global.t('login.unRules'), trigger: 'blur' }],
  newPass:     [{ required: true, message: i18n.global.t('login.pwRules'), trigger: 'blur' }],
}

const resetData = () => {
  newData.id = props.user?.id ?? 0
  newData.oldPass = ''
  newData.newUsername = ''
  newData.newPass = ''
}

const closeModal = () => {
  resetData()
  emit('close')
}

const saveChanges = async () => {
  if (!formRef.value) return
  await formRef.value.validate((valid) => {
    if (valid) emit('save', { ...newData })
  })
}

watch(() => props.visible, (v) => { if (v) resetData() })
</script>
