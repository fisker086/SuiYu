import { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    component: () => import('layouts/LoginPage.vue')
  },
  {
    path: '/register',
    component: () => import('layouts/RegisterPage.vue')
  },
  {
    path: '/',
    component: () => import('layouts/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: '/dashboard',
        name: 'dashboard',
        component: () => import('pages/DashboardPage.vue'),
        meta: { requireAuth: true, title: '仪表盘' }
      },
      /** 群聊：/chat/group/<session_id>；单聊：/chat/<agent public_id | 数字 id | 单聊 session uuid> */
      {
        path: '/chat/group/:sessionId',
        name: 'chat-group',
        component: () => import('pages/ChatPage.vue'),
        meta: { requireAuth: true, title: '对话' }
      },
      {
        path: '/chat/:agentId?',
        name: 'chat',
        component: () => import('pages/ChatPage.vue'),
        meta: { requireAuth: true, title: '对话' }
      },
      {
        path: '/agents',
        name: 'agents',
        component: () => import('pages/AgentsPage.vue'),
        meta: { requireAuth: true, requiresAdmin: true, title: '智能体' }
      },
      {
        path: '/skills',
        name: 'skills',
        component: () => import('pages/SkillsPage.vue'),
        meta: { requireAuth: true, requiresAdmin: true, title: '技能' }
      },
      {
        path: '/mcp',
        name: 'mcp',
        component: () => import('pages/MCPPage.vue'),
        meta: { requireAuth: true, requiresAdmin: true, title: 'MCP' }
      },
      {
        path: '/workflows',
        name: 'workflows',
        component: () => import('pages/WorkflowsPage.vue'),
        meta: { requireAuth: true, requiresAdmin: true, title: '工作流' }
      },
      {
        path: '/schedules',
        name: 'schedules',
        component: () => import('pages/SchedulesPage.vue'),
        meta: { requireAuth: true, title: '定时任务' }
      },
      {
        path: '/channels',
        name: 'channels',
        component: () => import('pages/ChannelsPage.vue'),
        meta: { requireAuth: true, title: '消息通知' }
      },
      {
        path: '/larkbots',
        redirect: '/bots'
      },
      {
        path: '/bots',
        name: 'bots',
        component: () => import('pages/BotPage.vue'),
        meta: { requireAuth: true, requiresAdmin: true, title: '智能机器人' }
      },
      {
        path: '/audit-logs',
        name: 'audit-logs',
        component: () => import('pages/AuditLogsPage.vue'),
        meta: { requireAuth: true, requiresAdmin: true, title: '审计日志' }
      },
      {
        path: '/approvals',
        name: 'approvals',
        component: () => import('pages/ApprovalsPage.vue'),
        meta: { requireAuth: true, title: '审批管理' }
      },
      {
        path: '/roles',
        name: 'roles',
        /** 角色 CRUD + 智能体勾选；用户分配角色见「用户」菜单 */
        component: () => import('pages/RBACPage.vue'),
        meta: { requireAuth: true, requiresAdmin: true, title: '角色管理' }
      },
      {
        path: '/users',
        name: 'users',
        component: () => import('pages/UsersPage.vue'),
        meta: { requireAuth: true, requiresAdmin: true, title: '用户' }
      },
      {
        path: '/usage',
        name: 'usage',
        component: () => import('pages/UsagePage.vue'),
        meta: { requireAuth: true, requiresAdmin: true, title: '用量统计' }
      },
      {
        path: '/rbac',
        redirect: { name: 'roles' }
      },
      /** 资源级 rbac_permissions 配置页已下线；对外以 Agent + 角色内「智能体权限」为主 */
      {
        path: '/permissions',
        redirect: { name: 'roles' }
      },
      {
        path: '/user-roles',
        redirect: { name: 'users' }
      },
      {
        path: '/profile',
        name: 'profile',
        component: () => import('pages/ProfilePage.vue'),
        meta: { requireAuth: true, title: '个人资料' }
      }
    ]
  },
  {
    path: '/:catchAll(.*)*',
    component: () => import('pages/ErrorNotFound.vue')
  }
]

export default routes
