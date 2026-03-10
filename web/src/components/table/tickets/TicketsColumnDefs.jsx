import React from 'react';
import { Tag, Button, Space, Tooltip, Rating } from '@douyinfe/semi-ui';
import { timestamp2string } from '../../../helpers';
import {
  TICKET_STATUS,
  TICKET_STATUS_MAP,
  TICKET_PRIORITY_MAP,
  TICKET_CATEGORY_MAP,
} from '../../../constants/ticket.constants';

const renderTimestamp = (timestamp) => {
  if (!timestamp) return '-';
  return <>{timestamp2string(timestamp)}</>;
};

const renderStatus = (status, t) => {
  const statusConfig = TICKET_STATUS_MAP[status];
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

const renderPriority = (priority, t) => {
  const config = TICKET_PRIORITY_MAP[priority];
  if (config) {
    return (
      <Tag color={config.color} shape='circle'>
        {t(config.text)}
      </Tag>
    );
  }
  return '-';
};

const renderCategory = (category, t) => {
  const config = TICKET_CATEGORY_MAP[category];
  if (config) {
    return <span>{t(config.text)}</span>;
  }
  return '-';
};

export const getTicketsColumns = ({
  t,
  admin,
  openTicketDetail,
  openAssignModal,
  updateTicketStatus,
}) => {
  const columns = [
    {
      title: t('ID'),
      dataIndex: 'id',
      width: 60,
    },
    {
      title: t('标题'),
      dataIndex: 'title',
      render: (text, record) => (
        <Tooltip content={text}>
          <a
            onClick={() => openTicketDetail(record)}
            style={{ cursor: 'pointer', color: 'var(--semi-color-link)' }}
            className='truncate max-w-[200px] inline-block'
          >
            {text}
          </a>
        </Tooltip>
      ),
    },
    {
      title: t('分类'),
      dataIndex: 'category',
      width: 100,
      render: (text) => renderCategory(text, t),
    },
    {
      title: t('优先级'),
      dataIndex: 'priority',
      width: 80,
      render: (text) => renderPriority(text, t),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 90,
      render: (text) => renderStatus(text, t),
    },
  ];

  if (admin) {
    columns.push(
      {
        title: t('提交用户'),
        dataIndex: 'username',
        width: 100,
        render: (text, record) => (
          <Tooltip content={`ID: ${record.user_id}`}>
            <span>{text}</span>
          </Tooltip>
        ),
      },
      {
        title: t('分配给'),
        dataIndex: 'assigned_name',
        width: 100,
        render: (text) => text || <span style={{ color: 'var(--semi-color-text-2)' }}>{t('未分配')}</span>,
      },
    );
  }

  columns.push(
    {
      title: t('评分'),
      dataIndex: 'rating',
      width: 120,
      render: (text) =>
        text > 0 ? (
          <Rating value={text} disabled size='small' />
        ) : (
          <span style={{ color: 'var(--semi-color-text-2)' }}>-</span>
        ),
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_time',
      render: (text) => <div>{renderTimestamp(text)}</div>,
    },
    {
      title: t('更新时间'),
      dataIndex: 'updated_time',
      render: (text) => <div>{renderTimestamp(text)}</div>,
    },
    {
      title: '',
      dataIndex: 'operate',
      fixed: 'right',
      width: admin ? 200 : 80,
      render: (text, record) => {
        return (
          <Space>
            <Button
              type='tertiary'
              size='small'
              onClick={() => openTicketDetail(record)}
            >
              {t('查看')}
            </Button>
            {admin && record.status !== TICKET_STATUS.CLOSED && (
              <>
                <Button
                  size='small'
                  onClick={() => openAssignModal(record)}
                >
                  {t('分配')}
                </Button>
                {record.status !== TICKET_STATUS.RESOLVED && (
                  <Button
                    type='primary'
                    size='small'
                    onClick={() =>
                      updateTicketStatus(record.id, TICKET_STATUS.RESOLVED)
                    }
                  >
                    {t('解决')}
                  </Button>
                )}
              </>
            )}
          </Space>
        );
      },
    },
  );

  return columns;
};
