<script setup lang="ts">
// 使用 Vue 3.4+ defineModel 实现极致简洁的双向绑定
const model = defineModel<boolean>({ default: false })

// disabled：禁用时不响应点击，并以降透明 + 禁用光标提示不可操作
// （用于「启用 TProxy 时禁止修改本机流量代理」等场景）。
const props = withDefaults(defineProps<{ disabled?: boolean }>(), { disabled: false })

const toggle = () => {
  if (props.disabled) return
  model.value = !model.value
}
</script>

<template>
  <button
    type="button"
    :disabled="disabled"
    @click="toggle"
    class="w-10 h-6 flex items-center rounded-full p-0.5 transition-all outline-none duration-200"
    :class="[
      model ? 'bg-accent justify-end' : 'bg-slate-200 dark:bg-slate-700 justify-start',
      disabled ? 'opacity-40 cursor-not-allowed' : ''
    ]"
  >
    <span class="w-5 h-5 rounded-full bg-white shadow-md transition-transform duration-200"></span>
  </button>
</template>
