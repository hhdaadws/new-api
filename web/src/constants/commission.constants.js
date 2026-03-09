export const COMMISSION_STATUS = {
  PENDING: 1,
  APPROVED: 2,
  REJECTED: 3,
};

export const COMMISSION_STATUS_MAP = {
  [COMMISSION_STATUS.PENDING]: {
    color: 'orange',
    text: '待审核',
  },
  [COMMISSION_STATUS.APPROVED]: {
    color: 'green',
    text: '已通过',
  },
  [COMMISSION_STATUS.REJECTED]: {
    color: 'red',
    text: '已拒绝',
  },
};
