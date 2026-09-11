import { computed, onMounted, ref, watch } from "vue";
import { Message } from "@arco-design/web-vue";

// HttpSpider / HeadlessSpider 共用的 tasks JSON 编辑逻辑：
// 解析已有配置 → 结构化表单深度写回 data.tasks；JSON 源码页的解析/应用/校验共用一套
export function useSpiderTasks(data, { initialTab = "" } = {}) {
  if (!data.value.tasks) {
    data.value.tasks = "[]";
  }

  // 解析已有 tasks JSON；失败则停在 JSON 源码页让用户先修复
  let initTasks = [];
  let initError = "";
  try {
    const parsed = JSON.parse(data.value.tasks || "[]");
    if (Array.isArray(parsed)) {
      initTasks = parsed;
    } else {
      initError = "tasks 必须是 JSON 数组";
    }
  } catch (e) {
    initError = e.message;
  }

  const activeKey = ref(initError ? "json" : initialTab);
  const tasks = ref(initTasks);

  // 表单编辑 → 写回插件配置（初始化赋值发生在 watch 建立之前，不会误触发）
  watch(tasks, (v) => {
    data.value.tasks = JSON.stringify(v, null, 2);
  }, { deep: true });

  if (initError) {
    onMounted(() => Message.warning(`tasks JSON 解析失败（${initError}），请在本页修复后再使用表单`));
  }

  const enabledCount = computed(() => tasks.value.filter((t) => t.enable).length);

  const jsonSummary = computed(() => {
    try {
      const parsed = JSON.parse(data.value.tasks || "[]");
      if (!Array.isArray(parsed)) return "tasks 必须是 JSON 数组";
      return `共 ${parsed.length} 个任务，${parsed.filter((t) => t.enable).length} 个启用`;
    } catch (e) {
      return "JSON 格式错误：" + e.message;
    }
  });

  // 切换页签：离开 JSON 源码页时自动解析应用，失败则拉回
  function onTabChange(key) {
    if (key !== "json") {
      applyJSON(true);
    }
  }

  function parseJSON() {
    const parsed = JSON.parse(data.value.tasks || "[]");
    if (!Array.isArray(parsed)) {
      throw new Error("tasks 必须是 JSON 数组");
    }
    return parsed;
  }

  // JSON → 表单。silent 模式（切页触发）下 JSON 与表单一致时不打扰用户
  function applyJSON(silent) {
    let parsed;
    try {
      parsed = parseJSON();
    } catch (e) {
      activeKey.value = "json";
      Message.error("tasks 解析失败：" + e.message);
      return;
    }
    const next = JSON.stringify(parsed, null, 2);
    if (silent && next === JSON.stringify(tasks.value, null, 2)) return;
    tasks.value = parsed;
    Message.success(`已应用到表单，共 ${parsed.length} 个任务`);
  }

  function formatJSON() {
    try {
      const parsed = parseJSON();
      data.value.tasks = JSON.stringify(parsed, null, 2);
      Message.success(`JSON 校验通过，共 ${parsed.length} 个任务`);
    } catch (e) {
      Message.error("tasks 解析失败：" + e.message);
    }
  }

  return { activeKey, tasks, enabledCount, jsonSummary, onTabChange, applyJSON, formatJSON };
}
