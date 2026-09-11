<template>
  <div>
    <a-input
      :id="inputId"
      v-model="text"
      :placeholder="placeholder"
      :status="regexError ? 'error' : undefined"
      :aria-describedby="regexError ? errorId : undefined"
      size="small"
      allow-clear
      @update:model-value="onInput"
    />
    <p v-if="regexError" :id="errorId" role="alert" class="text-xs text-red-500 mt-1 mb-0">
      {{ regexError }}
    </p>
  </div>
</template>
<script setup>
import { computed, ref, watch } from "vue";

const props = defineProps({
  attr: { type: String, default: "" },
  regex: { type: String, default: "" },
  placeholder: { type: String, default: "高级：@属性 /正则/，如 @style /url\\(([^)]+)\\)/" },
});
const emit = defineEmits(["update:attr", "update:regex"]);

// 供 aria-describedby 关联（页面内可能同时存在多个实例）
const inputId = `adv-${Math.random().toString(36).slice(2, 9)}`;
const errorId = `${inputId}-err`;

function format(attr, regex) {
  let s = "";
  if (attr) s += "@" + attr;
  if (regex) s += (s ? " " : "") + "/" + regex + "/";
  return s;
}
function parse(input) {
  const out = { attr: "", regex: "" };
  const s = (input || "").trim();
  const m = s.match(/@([^\s/]+)/);
  if (m) out.attr = m[1] === "auto" ? "" : m[1];
  const r = s.match(/\/(.+)\//);
  if (r) out.regex = r[1];
  return out;
}

const text = ref(format(props.attr, props.regex));
watch(() => format(props.attr, props.regex), (v) => { if (v !== text.value) text.value = v; });

// 内联校验（on-input 即时反馈，非仅提交时）：正则部分非法时就地报错并定位到该字段
const regexError = computed(() => {
  if (!props.regex) return "";
  try {
    // eslint-disable-next-line no-new
    new RegExp(props.regex);
    return "";
  } catch (e) {
    return "正则语法错误：" + String(e.message || e).replace(/^Invalid regular expression:\s*/, "");
  }
});

function onInput(v) {
  text.value = v;
  const parsed = parse(v);
  emit("update:attr", parsed.attr);
  emit("update:regex", parsed.regex);
}
</script>
