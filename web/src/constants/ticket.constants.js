export const TICKET_STATUS = {
  OPEN: 1,
  IN_PROGRESS: 2,
  RESOLVED: 3,
  CLOSED: 4,
};

export const TICKET_STATUS_MAP = {
  [TICKET_STATUS.OPEN]: {
    color: 'blue',
    text: '待处理',
  },
  [TICKET_STATUS.IN_PROGRESS]: {
    color: 'orange',
    text: '处理中',
  },
  [TICKET_STATUS.RESOLVED]: {
    color: 'green',
    text: '已解决',
  },
  [TICKET_STATUS.CLOSED]: {
    color: 'grey',
    text: '已关闭',
  },
};

export const TICKET_PRIORITY = {
  LOW: 1,
  MEDIUM: 2,
  HIGH: 3,
  URGENT: 4,
};

export const TICKET_PRIORITY_MAP = {
  [TICKET_PRIORITY.LOW]: {
    color: 'grey',
    text: '低',
  },
  [TICKET_PRIORITY.MEDIUM]: {
    color: 'blue',
    text: '中',
  },
  [TICKET_PRIORITY.HIGH]: {
    color: 'orange',
    text: '高',
  },
  [TICKET_PRIORITY.URGENT]: {
    color: 'red',
    text: '紧急',
  },
};

export const TICKET_CATEGORY = {
  ACCOUNT: 1,
  BILLING: 2,
  TECHNICAL: 3,
  FEATURE_REQUEST: 4,
  OTHER: 5,
};

export const TICKET_CATEGORY_MAP = {
  [TICKET_CATEGORY.ACCOUNT]: {
    text: '账户问题',
  },
  [TICKET_CATEGORY.BILLING]: {
    text: '计费问题',
  },
  [TICKET_CATEGORY.TECHNICAL]: {
    text: '技术支持',
  },
  [TICKET_CATEGORY.FEATURE_REQUEST]: {
    text: '功能建议',
  },
  [TICKET_CATEGORY.OTHER]: {
    text: '其他',
  },
};
