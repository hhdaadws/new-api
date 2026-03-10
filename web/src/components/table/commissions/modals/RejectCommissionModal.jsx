import React, { useState } from 'react';
import { Modal, TextArea } from '@douyinfe/semi-ui';

const RejectCommissionModal = ({
  visible,
  onCancel,
  onConfirm,
  record,
  isBatch,
  count,
  t,
}) => {
  const [remark, setRemark] = useState('');
  const [loading, setLoading] = useState(false);

  const handleOk = async () => {
    setLoading(true);
    await onConfirm(remark);
    setRemark('');
    setLoading(false);
  };

  const handleCancel = () => {
    setRemark('');
    onCancel();
  };

  const title = isBatch
    ? t('批量拒绝返佣记录')
    : t('拒绝返佣记录');

  const content = isBatch
    ? t('将拒绝 {{count}} 条所选记录', { count })
    : record
      ? t('确定拒绝该返佣申请？')
      : '';

  return (
    <Modal
      title={title}
      visible={visible}
      onOk={handleOk}
      onCancel={handleCancel}
      okText={t('确定')}
      cancelText={t('取消')}
      confirmLoading={loading}
      closeOnEsc={true}
    >
      <p style={{ marginBottom: 12 }}>{content}</p>
      <TextArea
        value={remark}
        onChange={setRemark}
        placeholder={t('拒绝原因（选填）')}
        autosize={{ minRows: 2, maxRows: 4 }}
      />
    </Modal>
  );
};

export default RejectCommissionModal;
