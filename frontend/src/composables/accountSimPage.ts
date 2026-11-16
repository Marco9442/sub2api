import type { InjectionKey } from 'vue'

/** 账号管理 #2 把「配置」放进更多操作。真实账号页不提供这个函数，菜单项不出现。 */
export const openAccountSimConfigKey: InjectionKey<() => void> = Symbol('openAccountSimConfig')

/** 账号管理 #2 不显示 Gemini 用量列的本地模拟说明。真实页面不提供，说明仍在。 */
export const hideGeminiSimulatedNoteKey: InjectionKey<boolean> = Symbol('hideGeminiSimulatedNote')
