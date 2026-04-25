export const sshNode = {
  value: 'ssh',
  label: 'SSH Execute',
  icon: 'terminal',
  color: '#4caf50',
  desc: 'Remote command execution',
  category: 'ops'
}

export const defaultConfig = {
  host: '',
  port: 22,
  username: '',
  password: '',
  command: '',
  timeout: 30
}
