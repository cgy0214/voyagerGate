// 全局响应式状态仓库：以后端 Snapshot 为唯一数据源，订阅 Wails 事件增量更新。
import { reactive } from 'vue'
import { EventsOn } from '../wailsjs/runtime/runtime'
import * as App from '../wailsjs/go/main/App'
import { setLocale } from './i18n'

export const state = reactive({
  snapshot: null,      // 后端全量快照
  current: null,       // 当前环境对象（快照的派生，响应式）
  selected: '',        // 选中服务名
  logPaused: false,    // 日志暂停
  logs: [],            // 前端展示日志（新→旧，上限 50）
  toastMsg: '',
  toastType: 'ok',     // ok（成功，绿色描边 3s）| err（失败，红色描边 5s）
  version: '',         // 产品版本（后端返回，形如 v1.0.0）
})

let _prevEnvName = ''
let _toastTimer = null

// 服务排序：在线 > 手动添加 > 注册中心（与 ServiceList 展示顺序一致）
function sortServices(list) {
  return list.slice().sort((a, b) => {
    if (a.online !== b.online) return a.online ? -1 : 1
    return (a.src === 'man' ? 0 : 1) - (b.src === 'man' ? 0 : 1)
  })
}

// applySnapshot 应用快照（保留前端展示态：日志列表、暂停、选中服务）
export function applySnapshot(snap) {
  state.snapshot = snap
  const env = snap.envs.find(e => e.name === snap.current) || snap.envs[0] || null
  // 环境切换 / 未选中 / 选中服务被删除 → 校正为排序后首个（在线优先）
  if (env && (env.name !== _prevEnvName || !state.selected || !env.services.some(s => s.name === state.selected))) {
    state.selected = sortServices(env.services)[0]?.name || ''
  }
  _prevEnvName = env ? env.name : ''
  state.current = env
}

// init 启动初始化：拉取快照并订阅后端事件
export async function init() {
  EventsOn('app:snapshot', snap => { applySnapshot(snap) })
  EventsOn('app:log', e => {
    if (!state.logPaused) {
      state.logs.unshift(e)
      if (state.logs.length > 50) state.logs.pop()
    }
  })
  EventsOn('app:stats', s => {
    if (state.snapshot) {
      state.snapshot.reqCount = s.reqCount
      state.snapshot.avgMs = s.avgMs
    }
  })
  applySnapshot(await App.Snapshot())
  state.logs = state.snapshot?.logs || []
  state.version = await App.GetVersion()
  // 以后端配置中的语言为准（config.yaml 持久化）
  setLocale(state.snapshot?.lang || 'zh')
}

// toast 底部居中玻璃气泡：ok 成功（绿色描边 3s）/ err 失败（红色描边 5s）
export function toast(msg, type = 'ok') {
  state.toastMsg = msg
  state.toastType = type
  clearTimeout(_toastTimer)
  _toastTimer = setTimeout(() => { state.toastMsg = '' }, type === 'err' ? 5000 : 3000)
}

// run 执行绑定调用：失败时 toast 错误信息（wails 返回 rejected Promise）
export async function run(fn, okMsg) {
  try {
    const r = await fn()
    if (okMsg) toast(okMsg)
    return r
  } catch (e) {
    toast(String(e && e.message ? e.message : e), 'err')
    throw e
  }
}

// cur 当前环境（可能为 null）
export function cur() { return state.current }

// rulesOf 某服务的全部规则（保持后端顺序）
export function rulesOf(svc) {
  return (state.current?.rules || []).filter(r => r.svc === svc)
}

export { App }
