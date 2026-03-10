import React, { useState, useEffect, useRef } from 'react';
import {
  SideSheet,
  Tag,
  Button,
  Input,
  Typography,
  Spin,
  Rating,
  Select,
  Divider,
  Modal,
  Avatar,
} from '@douyinfe/semi-ui';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { timestamp2string } from '../../../../helpers';
import {
  TICKET_STATUS,
  TICKET_STATUS_MAP,
  TICKET_PRIORITY_MAP,
  TICKET_CATEGORY_MAP,
} from '../../../../constants/ticket.constants';
import ErrorBoundary from '../../../common/ErrorBoundary';

const { Text, Title } = Typography;
const { TextArea } = Input;

const TicketDetailModal = ({
  visible,
  onClose,
  ticket,
  admin,
  loadTicketDetail,
  addMessage,
  closeTicket,
  rateTicket,
  updateTicketStatus,
  openAssignModal,
  refresh,
  t,
}) => {
  const isMobile = useIsMobile();
  const [detail, setDetail] = useState(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [replyContent, setReplyContent] = useState('');
  const [replying, setReplying] = useState(false);
  const [ratingValue, setRatingValue] = useState(0);
  const messagesEndRef = useRef(null);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (visible && ticket) {
      loadDetail();
    } else {
      setDetail(null);
      setReplyContent('');
      setRatingValue(0);
    }
  }, [visible, ticket]);

  const loadDetail = async () => {
    if (!ticket) return;
    setDetailLoading(true);
    try {
      const data = await loadTicketDetail(ticket.id);
      if (!mountedRef.current) return;
      if (data) {
        setDetail(data);
        setRatingValue(data.ticket?.rating || 0);
      }
    } catch (error) {
      console.error('Failed to load ticket detail:', error);
    } finally {
      if (mountedRef.current) setDetailLoading(false);
    }
  };

  const handleSendReply = async () => {
    if (!replyContent.trim() || !ticket) return;
    setReplying(true);
    try {
      const success = await addMessage(ticket.id, replyContent.trim());
      if (success) {
        setReplyContent('');
        await loadDetail();
      }
    } catch (error) {
      console.error('Failed to send reply:', error);
    } finally {
      if (mountedRef.current) setReplying(false);
    }
  };

  const handleClose = () => {
    Modal.confirm({
      title: t('确认关闭'),
      content: t('关闭后将无法继续回复，确定关闭此工单？'),
      onOk: async () => {
        try {
          const success = await closeTicket(ticket.id);
          if (success) {
            await loadDetail();
            refresh();
          }
        } catch (error) {
          console.error('Failed to close ticket:', error);
        }
      },
    });
  };

  const handleRate = async (value) => {
    setRatingValue(value);
    try {
      await rateTicket(ticket.id, value);
      await loadDetail();
    } catch (error) {
      console.error('Failed to rate ticket:', error);
    }
  };

  const handleStatusChange = async (status) => {
    try {
      await updateTicketStatus(ticket.id, status);
      await loadDetail();
    } catch (error) {
      console.error('Failed to update ticket status:', error);
    }
  };

  const ticketData = detail?.ticket || ticket;
  const messages = Array.isArray(detail?.messages) ? detail.messages : [];
  const isClosed = ticketData?.status === TICKET_STATUS.CLOSED;
  const isResolved = ticketData?.status === TICKET_STATUS.RESOLVED;
  const canRate = !admin && (isClosed || isResolved) && ticketData?.rating === 0;
  const canReply = !isClosed;

  const statusOptions = Object.entries(TICKET_STATUS_MAP).map(
    ([key, config]) => ({
      value: parseInt(key),
      label: t(config.text),
    }),
  );

  return (
    <SideSheet
      title={
        <div className='flex items-center gap-2'>
          <span>{t('工单详情')}</span>
          {ticketData && (
            <Tag color='grey' size='small'>
              #{ticketData.id}
            </Tag>
          )}
        </div>
      }
      visible={visible}
      onCancel={onClose}
      width={isMobile ? '100%' : 680}
      bodyStyle={{ padding: 0 }}
    >
      <ErrorBoundary>
        {detailLoading ? (
          <div className='flex justify-center items-center h-64'>
            <Spin size='large' />
          </div>
        ) : ticketData ? (
          <div className='flex flex-col h-full'>
            {/* 工单头部信息 */}
            <div className='p-4 border-b border-gray-200 dark:border-gray-700'>
              <Title heading={5} className='mb-2'>
                {ticketData.title}
              </Title>
              <div className='flex flex-wrap gap-2 mb-2'>
                <Tag
                  color={TICKET_STATUS_MAP[ticketData.status]?.color || 'grey'}
                  shape='circle'
                >
                  {t(TICKET_STATUS_MAP[ticketData.status]?.text || '未知')}
                </Tag>
                <Tag
                  color={TICKET_PRIORITY_MAP[ticketData.priority]?.color || 'grey'}
                  shape='circle'
                >
                  {t(TICKET_PRIORITY_MAP[ticketData.priority]?.text || '未知')}
                </Tag>
                <Tag shape='circle'>
                  {t(TICKET_CATEGORY_MAP[ticketData.category]?.text || '其他')}
                </Tag>
              </div>
              <div className='flex flex-wrap gap-4 text-sm' style={{ color: 'var(--semi-color-text-2)' }}>
                <span>
                  {t('创建时间')}: {timestamp2string(ticketData.created_time)}
                </span>
                {ticketData.assigned_name && (
                  <span>
                    {t('分配给')}: {ticketData.assigned_name}
                  </span>
                )}
                {ticketData.username && admin && (
                  <span>
                    {t('提交用户')}: {ticketData.username}
                  </span>
                )}
              </div>

              {/* 管理员操作栏 */}
              {admin && !isClosed && (
                <div className='flex flex-wrap gap-2 mt-3'>
                  <Select
                    value={ticketData.status}
                    onChange={handleStatusChange}
                    optionList={statusOptions}
                    size='small'
                    style={{ width: 120 }}
                  />
                  <Button
                    size='small'
                    onClick={() => openAssignModal(ticketData)}
                  >
                    {t('分配工单')}
                  </Button>
                </div>
              )}

              {/* 用户操作栏 */}
              {!admin && !isClosed && (
                <div className='mt-3'>
                  <Button
                    type='danger'
                    size='small'
                    onClick={handleClose}
                  >
                    {t('关闭工单')}
                  </Button>
                </div>
              )}
            </div>

            {/* 初始内容 */}
            <div className='p-4 border-b border-gray-200 dark:border-gray-700' style={{ backgroundColor: 'var(--semi-color-fill-0)' }}>
              <div className='flex items-center gap-2 mb-2'>
                <Avatar size='extra-small' color='blue'>
                  {ticketData.username?.[0]?.toUpperCase() || 'U'}
                </Avatar>
                <Text strong>{ticketData.username}</Text>
                <Text type='tertiary' size='small'>
                  {timestamp2string(ticketData.created_time)}
                </Text>
              </div>
              <div
                className='whitespace-pre-wrap'
                style={{ color: 'var(--semi-color-text-0)' }}
              >
                {ticketData.content}
              </div>
            </div>

            {/* 消息列表 */}
            <div
              className='flex-1 overflow-y-auto p-4'
              style={{ maxHeight: isMobile ? '40vh' : '45vh' }}
            >
              {messages.length > 0 ? (
                <div className='space-y-3'>
                  {messages.map((msg) => {
                    const isAdminMsg = msg.role >= 10;
                    return (
                      <div
                        key={msg.id}
                        className='p-3 rounded-lg'
                        style={{
                          backgroundColor: isAdminMsg
                            ? 'var(--semi-color-primary-light-default)'
                            : 'var(--semi-color-fill-0)',
                        }}
                      >
                        <div className='flex items-center gap-2 mb-1'>
                          <Avatar
                            size='extra-small'
                            color={isAdminMsg ? 'green' : 'blue'}
                          >
                            {msg.username?.[0]?.toUpperCase() || 'U'}
                          </Avatar>
                          <Text strong>{msg.username}</Text>
                          {isAdminMsg && (
                            <Tag color='green' size='small'>
                              {t('管理员')}
                            </Tag>
                          )}
                          <Text type='tertiary' size='small'>
                            {timestamp2string(msg.created_time)}
                          </Text>
                        </div>
                        <div
                          className='whitespace-pre-wrap ml-8'
                          style={{ color: 'var(--semi-color-text-0)' }}
                        >
                          {msg.content}
                        </div>
                      </div>
                    );
                  })}
                  <div ref={messagesEndRef} />
                </div>
              ) : (
                <div
                  className='text-center py-8'
                  style={{ color: 'var(--semi-color-text-2)' }}
                >
                  {t('暂无回复')}
                </div>
              )}
            </div>

            <Divider margin={0} />

            {/* 评分区域 */}
            {canRate && (
              <div className='p-4 border-b border-gray-200 dark:border-gray-700'>
                <div className='flex items-center gap-3'>
                  <Text>{t('评价此工单')}:</Text>
                  <Rating value={ratingValue} onChange={handleRate} />
                </div>
              </div>
            )}

            {/* 已评分显示 */}
            {ticketData.rating > 0 && (
              <div className='p-4 border-b border-gray-200 dark:border-gray-700'>
                <div className='flex items-center gap-3'>
                  <Text>{t('用户评分')}:</Text>
                  <Rating value={ticketData.rating} disabled />
                </div>
              </div>
            )}

            {/* 回复输入区 */}
            {canReply && (
              <div className='p-4'>
                <TextArea
                  value={replyContent}
                  onChange={setReplyContent}
                  placeholder={t('输入回复内容...')}
                  rows={3}
                  autosize={{ minRows: 2, maxRows: 6 }}
                />
                <div className='flex justify-end mt-2'>
                  <Button
                    type='primary'
                    loading={replying}
                    disabled={!replyContent.trim()}
                    onClick={handleSendReply}
                  >
                    {t('发送回复')}
                  </Button>
                </div>
              </div>
            )}

            {isClosed && !canRate && ticketData.rating === 0 && (
              <div
                className='p-4 text-center'
                style={{ color: 'var(--semi-color-text-2)' }}
              >
                {t('工单已关闭')}
              </div>
            )}
          </div>
        ) : null}
      </ErrorBoundary>
    </SideSheet>
  );
};

export default TicketDetailModal;
