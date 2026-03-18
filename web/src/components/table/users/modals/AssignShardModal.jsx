import React, { useState, useEffect, useCallback } from 'react';
import {
  Modal,
  Select,
  Typography,
  Tag,
  Space,
  Banner,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

const AssignShardModal = ({ visible, user, onClose, onSuccess }) => {
  const { t } = useTranslation();
  const [shards, setShards] = useState([]);
  const [selectedShard, setSelectedShard] = useState('');
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const loadShards = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/group_shard/');
      if (res.data.success) {
        setShards(res.data.data || []);
      }
    } catch {
      // ignore
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    if (visible) {
      loadShards();
      setSelectedShard('');
    }
  }, [visible, loadShards]);

  const handleSubmit = async () => {
    if (!selectedShard || !user) return;
    setSubmitting(true);
    try {
      const res = await API.post('/api/group_shard/assign_user', {
        user_id: user.id,
        shard_group: selectedShard,
      });
      if (res.data.success) {
        showSuccess(res.data.message || t('分配成功'));
        onSuccess?.();
      } else {
        showError(res.data.message);
      }
    } catch {
      showError(t('分配失败'));
    }
    setSubmitting(false);
  };

  // Group shards by parent group
  const groupedShards = shards.reduce((acc, shard) => {
    if (!acc[shard.parent_group]) {
      acc[shard.parent_group] = [];
    }
    acc[shard.parent_group].push(shard);
    return acc;
  }, {});

  return (
    <Modal
      title={t('分配用户到分片')}
      visible={visible}
      onCancel={onClose}
      onOk={handleSubmit}
      okButtonProps={{ loading: submitting, disabled: !selectedShard }}
      okText={t('确认分配')}
      cancelText={t('取消')}
      closeOnEsc
    >
      {user && (
        <>
          <Banner
            type='info'
            description={
              <span>
                {t('用户')}: <Text strong>{user.username}</Text> (ID:{' '}
                {user.id}) | {t('当前分组')}: <Tag>{user.group}</Tag>
              </span>
            }
            style={{ marginBottom: 16 }}
            closeIcon={null}
          />
          <Select
            placeholder={t('选择目标分片')}
            value={selectedShard}
            onChange={setSelectedShard}
            loading={loading}
            style={{ width: '100%' }}
            optionList={Object.entries(groupedShards).flatMap(
              ([parent, parentShards]) => [
                {
                  label: (
                    <Text type='tertiary' size='small'>
                      {parent}
                    </Text>
                  ),
                  value: `__group_${parent}`,
                  disabled: true,
                },
                ...parentShards.map((s) => ({
                  label: (
                    <Space>
                      <span>{s.shard_group}</span>
                      <Text type='tertiary' size='small'>
                        ({s.current_users}/{s.max_users === 0 ? '∞' : s.max_users})
                      </Text>
                      {!s.enabled && (
                        <Tag color='grey' size='small'>
                          {t('禁用')}
                        </Tag>
                      )}
                    </Space>
                  ),
                  value: s.shard_group,
                })),
              ]
            )}
          />
        </>
      )}
    </Modal>
  );
};

export default AssignShardModal;
