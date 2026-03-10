import React, { useRef, useState } from 'react';
import { SideSheet, Form, Button, Select } from '@douyinfe/semi-ui';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import {
  TICKET_PRIORITY,
  TICKET_PRIORITY_MAP,
  TICKET_CATEGORY,
  TICKET_CATEGORY_MAP,
} from '../../../../constants/ticket.constants';

const CreateTicketModal = ({ visible, onClose, onSubmit, t }) => {
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);

  const priorityOptions = Object.entries(TICKET_PRIORITY_MAP).map(
    ([key, config]) => ({
      value: parseInt(key),
      label: t(config.text),
    }),
  );

  const categoryOptions = Object.entries(TICKET_CATEGORY_MAP).map(
    ([key, config]) => ({
      value: parseInt(key),
      label: t(config.text),
    }),
  );

  const handleSubmit = async () => {
    if (!formApiRef.current) return;
    const values = formApiRef.current.getValues();

    if (!values.title || !values.title.trim()) {
      return;
    }
    if (!values.content || !values.content.trim()) {
      return;
    }

    setLoading(true);
    const success = await onSubmit({
      title: values.title.trim(),
      content: values.content.trim(),
      category: values.category || TICKET_CATEGORY.OTHER,
      priority: values.priority || TICKET_PRIORITY.MEDIUM,
    });
    setLoading(false);

    if (success && formApiRef.current) {
      formApiRef.current.reset();
    }
  };

  return (
    <SideSheet
      title={t('创建工单')}
      visible={visible}
      onCancel={onClose}
      width={isMobile ? '100%' : 600}
      footer={
        <div className='flex justify-end gap-2'>
          <Button onClick={onClose}>{t('取消')}</Button>
          <Button type='primary' loading={loading} onClick={handleSubmit}>
            {t('提交')}
          </Button>
        </div>
      }
    >
      <Form
        getFormApi={(api) => {
          formApiRef.current = api;
        }}
        initValues={{
          priority: TICKET_PRIORITY.MEDIUM,
          category: TICKET_CATEGORY.OTHER,
        }}
        layout='vertical'
      >
        <Form.Input
          field='title'
          label={t('标题')}
          placeholder={t('请输入工单标题')}
          rules={[{ required: true, message: t('请输入工单标题') }]}
        />

        <div className='flex gap-4'>
          <Form.Select
            field='category'
            label={t('分类')}
            optionList={categoryOptions}
            style={{ width: '50%' }}
          />
          <Form.Select
            field='priority'
            label={t('优先级')}
            optionList={priorityOptions}
            style={{ width: '50%' }}
          />
        </div>

        <Form.TextArea
          field='content'
          label={t('内容')}
          placeholder={t('请描述您的问题或需求')}
          rows={6}
          rules={[{ required: true, message: t('请输入工单内容') }]}
        />
      </Form>
    </SideSheet>
  );
};

export default CreateTicketModal;
