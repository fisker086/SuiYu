import type { NodeConfig } from './types'

interface FieldComponent {
  name: string
  value: unknown
  component: string
  props: Record<string, unknown>
}

type FieldGetter = (
  config: NodeConfig,
  updateConfig: (config: NodeConfig) => void,
  t: (key: string) => string
) => FieldComponent[]

function createInput (
  name: string,
  label: string,
  configKey: string,
  config: NodeConfig,
  updateConfig: (config: NodeConfig) => void,
  extraProps: Record<string, unknown> = {}
): FieldComponent {
  return {
    name,
    value: config[configKey] ?? '',
    component: 'q-input',
    props: {
      label,
      outlined: true,
      dense: true,
      'model-value': config[configKey] ?? '',
      ...extraProps,
      'onUpdate:modelValue': (v: unknown) => {
        updateConfig({ ...config, [configKey]: v })
      }
    }
  }
}

function createSelect (
  name: string,
  label: string,
  configKey: string,
  config: NodeConfig,
  updateConfig: (config: NodeConfig) => void,
  options: Array<{ label: string; value: string }>,
  extraProps: Record<string, unknown> = {}
): FieldComponent {
  return {
    name,
    value: config[configKey] ?? '',
    component: 'q-select',
    props: {
      label,
      outlined: true,
      dense: true,
      options,
      'emit-value': true,
      'map-options': true,
      'model-value': config[configKey] ?? '',
      ...extraProps,
      'onUpdate:modelValue': (v: unknown) => {
        updateConfig({ ...config, [configKey]: v })
      }
    }
  }
}

const nodeFieldComponents: Record<string, FieldGetter> = {
  agent: (config, updateConfig, t) => [
    createInput(t('wfBindAgent'), t('wfBindAgent'), 'agent_id', config, updateConfig, {
      type: 'number',
      label: t('wfBindAgent'),
      'model-value': config.agent_id,
      'onUpdate:modelValue': (v: number) => updateConfig({ ...config, agent_id: v })
    }),
    createInput(t('wfPromptTemplate'), t('wfPromptTemplate'), 'prompt_template', config, updateConfig, {
      type: 'textarea',
      rows: 6,
      label: t('wfPromptTemplate'),
      'model-value': config.prompt_template,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, prompt_template: v })
    })
  ],

  llm: (config, updateConfig, t) => [
    createInput(t('wfLLMSystemPrompt'), t('wfLLMSystemPrompt'), 'system_prompt', config, updateConfig, {
      type: 'textarea',
      rows: 3,
      label: t('wfLLMSystemPrompt'),
      'model-value': config.system_prompt,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, system_prompt: v })
    }),
    createInput(t('wfLLMTemperature'), t('wfLLMTemperature'), 'temperature', config, updateConfig, {
      type: 'number',
      step: 0.1,
      min: 0,
      max: 2,
      label: t('wfLLMTemperature'),
      'model-value': config.temperature,
      'onUpdate:modelValue': (v: number) => updateConfig({ ...config, temperature: v })
    })
  ],

  http: (config, updateConfig, t) => [
    createSelect(t('wfHTTPMethod'), t('wfHTTPMethod'), 'method', config, updateConfig, [
      { label: 'GET', value: 'GET' },
      { label: 'POST', value: 'POST' },
      { label: 'PUT', value: 'PUT' },
      { label: 'DELETE', value: 'DELETE' },
      { label: 'PATCH', value: 'PATCH' }
    ]),
    createInput(t('wfHTTPURL'), t('wfHTTPURL'), 'url', config, updateConfig, {
      label: t('wfHTTPURL'),
      'model-value': config.url,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, url: v })
    }),
    createInput(t('wfHTTPHeaders'), t('wfHTTPHeaders'), 'headers', config, updateConfig, {
      type: 'textarea',
      rows: 2,
      label: t('wfHTTPHeaders'),
      'model-value': typeof config.headers === 'string' ? config.headers : JSON.stringify(config.headers, null, 2),
      'onUpdate:modelValue': (v: string) => {
        try {
          updateConfig({ ...config, headers: JSON.parse(v) })
        } catch {
          updateConfig({ ...config, headers: {} })
        }
      }
    }),
    createInput(t('wfHTTPBody'), t('wfHTTPBody'), 'body', config, updateConfig, {
      type: 'textarea',
      rows: 3,
      label: t('wfHTTPBody'),
      'model-value': config.body,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, body: v })
    })
  ],

  ssh: (config, updateConfig, t) => [
    createInput(t('wfSSHHost'), t('wfSSHHost'), 'host', config, updateConfig, {
      label: t('wfSSHHost'),
      'model-value': config.host,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, host: v })
    }),
    createInput(t('wfSSHPort'), t('wfSSHPort'), 'port', config, updateConfig, {
      type: 'number',
      label: t('wfSSHPort'),
      'model-value': config.port,
      'onUpdate:modelValue': (v: number) => updateConfig({ ...config, port: v })
    }),
    createInput(t('wfSSHUser'), t('wfSSHUser'), 'username', config, updateConfig, {
      label: t('wfSSHUser'),
      'model-value': config.username,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, username: v })
    }),
    createInput(t('wfSSHPassword'), t('wfSSHPassword'), 'password', config, updateConfig, {
      type: 'password',
      label: t('wfSSHPassword'),
      'model-value': config.password,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, password: v })
    }),
    createInput(t('wfSSHCommand'), t('wfSSHCommand'), 'command', config, updateConfig, {
      type: 'textarea',
      rows: 3,
      label: t('wfSSHCommand'),
      'model-value': config.command,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, command: v })
    }),
    createInput(t('wfSSHTimeout'), t('wfSSHTimeout'), 'timeout', config, updateConfig, {
      type: 'number',
      label: t('wfSSHTimeout'),
      'model-value': config.timeout,
      'onUpdate:modelValue': (v: number) => updateConfig({ ...config, timeout: v })
    })
  ],

  notify: (config, updateConfig, t) => [
    createSelect(t('wfNotifyChannel'), t('wfNotifyChannel'), 'channel', config, updateConfig, [
      { label: '钉钉 DingTalk', value: 'dingtalk' },
      { label: '飞书 Lark', value: 'lark' },
      { label: '企业微信 WeCom', value: 'wecom' },
      { label: '邮件 Email', value: 'email' }
    ]),
    createInput(t('wfNotifyTitle'), t('wfNotifyTitle'), 'title', config, updateConfig, {
      label: t('wfNotifyTitle'),
      'model-value': config.title,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, title: v })
    }),
    createInput(t('wfNotifyMessage'), t('wfNotifyMessage'), 'message', config, updateConfig, {
      type: 'textarea',
      rows: 3,
      label: t('wfNotifyMessage'),
      'model-value': config.message,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, message: v })
    }),
    createInput(t('wfNotifyReceivers'), t('wfNotifyReceivers'), 'receivers', config, updateConfig, {
      label: t('wfNotifyReceivers'),
      'model-value': config.receivers,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, receivers: v })
    })
  ],

  apitest: (config, updateConfig, t) => [
    createSelect(t('wfAPITestMethod'), t('wfAPITestMethod'), 'method', config, updateConfig, [
      { label: 'GET', value: 'GET' },
      { label: 'POST', value: 'POST' },
      { label: 'PUT', value: 'PUT' },
      { label: 'DELETE', value: 'DELETE' },
      { label: 'PATCH', value: 'PATCH' }
    ]),
    createInput(t('wfAPITestURL'), t('wfAPITestURL'), 'url', config, updateConfig, {
      label: t('wfAPITestURL'),
      'model-value': config.url,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, url: v })
    }),
    createInput(t('wfAPITestHeaders'), t('wfAPITestHeaders'), 'headers', config, updateConfig, {
      type: 'textarea',
      rows: 2,
      label: t('wfAPITestHeaders'),
      'model-value': typeof config.headers === 'string' ? config.headers : JSON.stringify(config.headers, null, 2),
      'onUpdate:modelValue': (v: string) => {
        try {
          updateConfig({ ...config, headers: JSON.parse(v) })
        } catch {
          updateConfig({ ...config, headers: {} })
        }
      }
    }),
    createInput(t('wfAPITestBody'), t('wfAPITestBody'), 'body', config, updateConfig, {
      type: 'textarea',
      rows: 3,
      label: t('wfAPITestBody'),
      'model-value': config.body,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, body: v })
    }),
    createInput(t('wfAPITestAssertions'), t('wfAPITestAssertions'), 'assertions', config, updateConfig, {
      type: 'textarea',
      rows: 3,
      label: t('wfAPITestAssertions'),
      'model-value': config.assertions,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, assertions: v })
    })
  ],

  datamask: (config, updateConfig, t) => [
    createInput(t('wfDataMaskFields'), t('wfDataMaskFields'), 'fields', config, updateConfig, {
      label: t('wfDataMaskFields'),
      'model-value': config.fields,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, fields: v })
    }),
    createSelect(t('wfDataMaskType'), t('wfDataMaskType'), 'mask_type', config, updateConfig, [
      { label: '手机号 phone', value: 'phone' },
      { label: '邮箱 email', value: 'email' },
      { label: '身份证 id_card', value: 'id_card' },
      { label: '银行卡 bank_card', value: 'bank_card' },
      { label: '自定义 custom', value: 'custom' }
    ]),
    createInput(t('wfDataMaskPattern'), t('wfDataMaskPattern'), 'pattern', config, updateConfig, {
      label: t('wfDataMaskPattern'),
      'model-value': config.pattern,
      'onUpdate:modelValue': (v: string) => updateConfig({ ...config, pattern: v })
    })
  ]
}

export { nodeFieldComponents }
export type { FieldGetter, FieldComponent }
