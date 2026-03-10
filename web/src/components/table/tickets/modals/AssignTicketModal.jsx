import React, { useState, useEffect } from 'react';
import { Modal, Select, Spin } from '@douyinfe/semi-ui';
import { API, showError } from '../../../../helpers';

const AssignTicketModal = ({ visible, onClose, ticket, onAssign, t }) => {
  const [admins, setAdmins] = useState([]);
  const [selectedAdmin, setSelectedAdmin] = useState(null);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (visible) {
      loadAdmins();
      setSelectedAdmin(ticket?.assigned_to || null);
    }
  }, [visible]);

  const loadAdmins = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/ticket/admins');
      const { success, message, data } = res.data;
      if (success) {
        setAdmins(data || []);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  const handleOk = async () => {
    if (!selectedAdmin || !ticket) return;
    const admin = admins.find((a) => a.id === selectedAdmin);
    if (!admin) return;

    setSubmitting(true);
    await onAssign(ticket.id, admin.id, admin.display_name || admin.username);
    setSubmitting(false);
  };

  const adminOptions = admins.map((admin) => ({
    value: admin.id,
    label: `${admin.display_name || admin.username} (ID: ${admin.id})`,
  }));

  return (
    <Modal
      title={t('分配工单')}
      visible={visible}
      onCancel={onClose}
      onOk={handleOk}
      okButtonProps={{ loading: submitting, disabled: !selectedAdmin }}
      okText={t('确认分配')}
      cancelText={t('取消')}
    >
      {loading ? (
        <div className='flex justify-center py-8'>
          <Spin />
        </div>
      ) : (
        <div>
          <div className='mb-2' style={{ color: 'var(--semi-color-text-1)' }}>
            {t('选择管理员')}:
          </div>
          <Select
            value={selectedAdmin}
            onChange={setSelectedAdmin}
            optionList={adminOptions}
            placeholder={t('请选择管理员')}
            style={{ width: '100%' }}
            filter
          />
        </div>
      )}
    </Modal>
  );
};

export default AssignTicketModal;
