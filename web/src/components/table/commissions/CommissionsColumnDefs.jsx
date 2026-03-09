import React from 'react';
import { Tag, Button, Space, Tooltip } from '@douyinfe/semi-ui';
import { renderQuota, timestamp2string } from '../../../helpers';
import {
  COMMISSION_STATUS,
  COMMISSION_STATUS_MAP,
} from '../../../constants/commission.constants';

const renderTimestamp = (timestamp) => {
  if (!timestamp) return '-';
  return <>{timestamp2string(timestamp)}</>;
};

const renderStatus = (status, t) => {
  const statusConfig = COMMISSION_STATUS_MAP[status];
  if (statusConfig) {
    return (
      <Tag color={statusConfig.color} shape='circle'>
        {t(statusConfig.text)}
      </Tag>
    );
  }
  return (
    <Tag color='grey' shape='circle'>
      {t('未知状态')}
    </Tag>
  );
};

export const getCommissionsColumns = ({
  t,
  approveCommission,
  showRejectCommissionModal,
}) => {
  return [
    {
      title: t('ID'),
      dataIndex: 'id',
      width: 60,
    },
    {
      title: t('邀请人'),
      dataIndex: 'inviter_username',
      render: (text, record) => (
        <Tooltip content={`ID: ${record.inviter_id}`}>
          <span>{text}</span>
        </Tooltip>
      ),
    },
    {
      title: t('被邀请人'),
      dataIndex: 'invitee_username',
      render: (text, record) => (
        <Tooltip content={`ID: ${record.user_id}`}>
          <span>{text}</span>
        </Tooltip>
      ),
    },
    {
      title: t('充值额度'),
      dataIndex: 'quota_added',
      render: (text) => (
        <Tag color='grey' shape='circle'>
          {renderQuota(parseInt(text))}
        </Tag>
      ),
    },
    {
      title: t('返佣比例'),
      dataIndex: 'rate',
      render: (text) => <span>{text}%</span>,
      width: 80,
    },
    {
      title: t('返佣额度'),
      dataIndex: 'commission',
      render: (text) => (
        <Tag color='blue' shape='circle'>
          {renderQuota(parseInt(text))}
        </Tag>
      ),
    },
    {
      title: t('订单序号'),
      dataIndex: 'order_number',
      width: 80,
      render: (text) => <span>{t('第 {{n}} 笔', { n: text })}</span>,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 90,
      render: (text) => renderStatus(text, t),
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_time',
      render: (text) => <div>{renderTimestamp(text)}</div>,
    },
    {
      title: t('审核时间'),
      dataIndex: 'reviewed_time',
      render: (text) => <div>{renderTimestamp(text)}</div>,
    },
    {
      title: t('拒绝原因'),
      dataIndex: 'remark',
      render: (text) =>
        text ? (
          <Tooltip content={text}>
            <span className='truncate max-w-[100px] inline-block'>{text}</span>
          </Tooltip>
        ) : (
          '-'
        ),
    },
    {
      title: '',
      dataIndex: 'operate',
      fixed: 'right',
      width: 150,
      render: (text, record) => {
        if (record.status !== COMMISSION_STATUS.PENDING) {
          return null;
        }
        return (
          <Space>
            <Button
              type='primary'
              size='small'
              onClick={() => approveCommission(record.id)}
            >
              {t('通过')}
            </Button>
            <Button
              type='danger'
              size='small'
              onClick={() => showRejectCommissionModal(record)}
            >
              {t('拒绝')}
            </Button>
          </Space>
        );
      },
    },
  ];
};
